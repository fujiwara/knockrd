package knockrd

import (
	"github.com/aws/aws-sdk-go/private/protocol/json/jsonutil"
)

func marshalJSON(s interface{}) ([]byte, error) {
	b, err := jsonutil.BuildJSON(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func JSONString(s interface{}) string {
	b, _ := marshalJSON(s)
	return string(b)
}
