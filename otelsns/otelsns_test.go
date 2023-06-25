package otelsns

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func TestSqsCarrierAttributes(t *testing.T) {
	input := sns.PublishInput{}
	carrier := NewCarrierAttributes(&input)

	// no keys

	if len(carrier.Keys()) != 0 {
		t.Errorf("expected empty carrier")
	}

	if value1 := carrier.Get("key1"); value1 != "" {
		t.Errorf("found unexpected key key1")
	}

	// add key1

	carrier.Set("key1", "value1")

	if len(carrier.Keys()) != 1 {
		t.Errorf("expected only one key")
	}

	if carrier.Get("key1") != "value1" {
		t.Errorf("wrong value for key1")
	}

	// change key1

	carrier.Set("key1", "value2")

	if len(carrier.Keys()) != 1 {
		t.Errorf("expected only one key")
	}

	if carrier.Get("key1") != "value2" {
		t.Errorf("wrong value for key1")
	}

	// add key2

	carrier.Set("key2", "value3")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value2" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value3" {
		t.Errorf("wrong value for key2")
	}

	// change key1

	carrier.Set("key1", "value11")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value3" {
		t.Errorf("wrong value for key2")
	}

	// change key2

	carrier.Set("key2", "value22")

	if len(carrier.Keys()) != 2 {
		t.Errorf("expected two keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value22" {
		t.Errorf("wrong value for key2")
	}

	// add key3

	carrier.Set("key3", "value3")

	if len(carrier.Keys()) != 3 {
		t.Errorf("expected three keys")
	}

	if carrier.Get("key1") != "value11" {
		t.Errorf("wrong value for key1")
	}

	if carrier.Get("key2") != "value22" {
		t.Errorf("wrong value for key2")
	}

	if carrier.Get("key3") != "value3" {
		t.Errorf("wrong value for key3")
	}
}
