package knockrd_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fujiwara/knockrd"
)

var dynamoDBStreamEventJSON = []byte(`
{
  "Records": [
    {
      "eventID": "1",
      "eventName": "INSERT",
      "eventVersion": "1.0",
      "eventSource": "aws:dynamodb",
      "awsRegion": "us-east-1",
      "dynamodb": {
        "Keys": {
          "Key": {
            "S": "198.51.100.1"
          }
        },
        "SequenceNumber": "111",
        "StreamViewType": "KEYS_ONLY"
      },
      "eventSourceARN": "stream-ARN"
    },
    {
      "eventID": "2",
      "eventName": "MODIFY",
      "eventVersion": "1.0",
      "eventSource": "aws:dynamodb",
      "awsRegion": "us-east-1",
      "dynamodb": {
        "Keys": {
          "Key": {
            "S": "2001:db8::1"
          }
        },
        "SequenceNumber": "222",
        "StreamViewType": "KEYS_ONLY"
      },
      "eventSourceARN": "stream-ARN"
    },
    {
      "eventID": "3",
      "eventName": "REMOVE",
      "eventVersion": "1.0",
      "eventSource": "aws:dynamodb",
      "awsRegion": "us-east-1",
      "dynamodb": {
        "Keys": {
          "Key": {
            "S": "198.51.100.1"
          }
        },
        "SequenceNumber": "333",
        "StreamViewType": "KEYS_ONLY"
      },
      "eventSourceARN": "stream-ARN"
    },
    {
      "eventID": "4",
      "eventName": "INSERT",
      "eventVersion": "1.0",
      "eventSource": "aws:dynamodb",
      "awsRegion": "us-east-1",
      "dynamodb": {
        "Keys": {
          "Key": {
            "S": "198.51.100.123"
          }
        },
        "SequenceNumber": "333",
        "StreamViewType": "KEYS_ONLY"
      },
      "eventSourceARN": "stream-ARN"
    }
  ]
}
`)

func TestStream(t *testing.T) {
	if !doTestBackend {
		t.Skip("skip stream test")
		return
	}
	handler := knockrd.NewStreamHandler(conf)
	if handler == nil {
		t.Error("faild to new stream handler")
	}
	var ev events.DynamoDBEvent
	if err := json.Unmarshal(dynamoDBStreamEventJSON, &ev); err != nil {
		t.Error(err)
	}
	if err := handler(context.Background(), ev); err != nil {
		t.Error(err)
	}
}
