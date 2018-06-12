package dns

import (
	"fmt"
	"testing"
)

func TestResolver_Query(t *testing.T) {
	resolver := DefaultResolver
	resolver.Server = "1.1.1.1:53"

	if result, err := resolver.Query("www.baidu.com", true, -1); err != nil {
		t.Error(err)
	} else {
		fmt.Println(result)
	}
}
