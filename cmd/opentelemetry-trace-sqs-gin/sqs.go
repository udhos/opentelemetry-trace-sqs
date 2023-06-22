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

			ctx := context.TODO()

			forward(ctx, app, msg)

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

func forward(ctx context.Context, app *application, msg types.Message) {

	const me = "forward"

	// send to SQS
	forwardSQS(ctx, app, msg)

	// send to HTTP
	errHTTP := backend(ctx, app, bytes.NewBufferString(*msg.Body))
	if errHTTP != nil {
		log.Printf("%s: %v", me, errHTTP)
	}
}

func forwardSQS(ctx context.Context, app *application, msg types.Message) {

	const me = "forwardSQS"

	input := &sqs.SendMessageInput{
		QueueUrl:          aws.String(app.config.queueURLOutput),
		DelaySeconds:      0, // 0..900
		MessageAttributes: msg.MessageAttributes,
		MessageBody:       msg.Body,
	}

	_, errSend := app.queueOutput.client.SendMessage(ctx, input)
	if errSend != nil {
		log.Printf("%s: MessageId: %s - SendMessage: error: %v",
			me, *msg.MessageId, errSend)
	}
}
