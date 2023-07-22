package upload

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMediaFiles_ScanBiludVideo(t *testing.T) {
	// mf := NewMediaFiles("./test")
	var resp UploadResp

	jsonData := []byte(`{
        "code": 0,
        "message": "0",
        "ttl": 1,
        "data": {
            "aid": ["SSN"],
            "bvid": "BV14k4y157GJ"
        }
    }`)

	if err := json.Unmarshal(jsonData, &resp); err != nil {
		fmt.Println(err)
		return
	}

	if resp.Code == 0 {
		t.Log("success")
	}

	fmt.Printf("%+v\n", resp)
}
