package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/udhos/boilerplate/awsconfig"
	"github.com/udhos/opentelemetry-trace-sqs/otelsqs"
	"go.opentelemetry.io/otel/codes"
)

type sqsQueue struct {
	client *sqs.Client
	URL    string
}

func newSqsClient(caller, queueURL, roleArn, roleSessionName, endpointURL string) sqsQueue {

	const me = "newSqsClient"

	region, errRegion := getRegion(queueURL)
	if errRegion != nil {
		log.Fatalf("%s: %s: error: %v", me, caller, errRegion)
	}

	awsConfOptions := awsconfig.Options{
		Region:          region,
		RoleArn:         roleArn,
		RoleSessionName: roleSessionName,
		EndpointURL:     endpointURL,
	}

	cfg, errAwsConfig := awsconfig.AwsConfig(awsConfOptions)
	if errAwsConfig != nil {
		log.Fatalf("%s: %s: aws config error: %v", me, caller, errRegion)
	}

	q := sqsQueue{
		client: sqs.NewFromConfig(cfg.AwsConfig),
		URL:    queueURL,
	}

	return q
}

func getRegion(queueURL string) (string, error) {
	fields := strings.SplitN(queueURL, ".", 3)
	if len(fields) < 3 {
		return "", fmt.Errorf("queueRegion: bad queue url=[%s]", queueURL)
	}
	region := fields[1]
	log.Printf("queueRegion=[%s]", region)
	return region, nil
}

func sqsListener(app *application) {

	const me = "sqsListener"

	q := app.queueInput

	debug := true

	const cooldown = 10 * time.Second

	input := &sqs.ReceiveMessageInput{
		QueueUrl: aws.String(q.URL),
		AttributeNames: []types.QueueAttributeName{
			"SentTimestamp",
		},
		MaxNumberOfMessages: 10, // 1..10
		MessageAttributeNames: []string{
			"All",
		},
		WaitTimeSeconds: 20, // 0..20
	}

	for {
		if debug {
			log.Printf("%s: ready: %s", me, q.URL)
		}

		//
		// read message from sqs queue
		//

		//m.receive.WithLabelValues(queueID).Inc()

		resp, errRecv := q.client.ReceiveMessage(context.TODO(), input)
		if errRecv != nil {
			log.Printf("%s: sqs.ReceiveMessage: error: %v, sleeping %v",
				me, errRecv, cooldown)
			time.Sleep(cooldown)
			continue
		}

		//
		// push messages into channel
		//

		count := len(resp.Messages)

		if debug {
			log.Printf("%s: sqs.ReceiveMessage: found %d messages", me, count)
		}

		if count == 0 {
			if debug {
				log.Printf("%s: empty receive, sleeping %v",
					me, cooldown)
			}
			// this cooldown prevents us from hammering the api on empty receives.
			// it shouldn't really on live aws api, but it does take place on
			// simulated apis.
			time.Sleep(cooldown)
			continue
		}

		for i, msg := range resp.Messages {
			if debug {
				log.Printf("%s: %d/%d MessageId: %s", me, i+1, count, *msg.MessageId)
			}

			sqsForward(app, msg)

			//
			// delete from source queue
			//

			inputDelete := &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(q.URL),
				ReceiptHandle: msg.ReceiptHandle,
			}
			_, errDelete := q.client.DeleteMessage(context.TODO(), inputDelete)
			if errDelete != nil {
				log.Printf("%s: MessageId: %s - sqs.DeleteMessage: error: %v, sleeping %v",
					me, *msg.MessageId, errDelete, cooldown)
				time.Sleep(cooldown)
			}
		}
	}

}

// sqsForward sends message to both SQS and HTTP.
// will retrieve traceID from sqsMessage,
// reset traceID back into sqsMessage (since incoming attr might differ from outgoing attr),
// and create a context with traceID for HTTP.
func sqsForward(app *application, sqsMessage types.Message) {

	const me = "sqsForward"

	ctx := otelsqs.ContextFromSqsMessageAttributes(&sqsMessage)

	ctxNew, span := app.tracer.Start(ctx, me)
	defer span.End()

	log.Printf("%s: traceID=%s", me, span.SpanContext().TraceID().String())

	//
	// send to SQS
	//
	sqsSend(ctxNew, app, sqsMessage)

	//
	// send to HTTP
	//
	errHTTP := httpBackend(ctxNew, app, bytes.NewBufferString(*sqsMessage.Body))
	if errHTTP != nil {
		m := fmt.Sprintf("%s: %v", me, errHTTP)
		log.Print(m)
		span.SetStatus(codes.Error, m)
	}
}

// sqsSend only submits message to SQS.
// attribute with traceID must have been set in sqsMessage.
func sqsSend(ctx context.Context, app *application, sqsMessage types.Message) {

	const me = "sqsSend"

	newCtx, span := app.tracer.Start(ctx, me)
	defer span.End()

	input := &sqs.SendMessageInput{
		QueueUrl:          aws.String(app.config.queueURLOutput),
		DelaySeconds:      0, // 0..900
		MessageAttributes: sqsMessage.MessageAttributes,
		MessageBody:       sqsMessage.Body,
	}

	_, errSend := app.queueOutput.client.SendMessage(newCtx, input)
	if errSend != nil {
		m := fmt.Sprintf("%s: MessageId: %s - SendMessage: error: %v",
			me, *sqsMessage.MessageId, errSend)
		log.Print(m)
		span.SetStatus(codes.Error, m)
	}
}
