package dns

import (
    "fmt"
    "testing"
)

func TestResolver_Query(t *testing.T) {
    resolver := DefaultResolver
    resolver.Server = "114.114.114.114:53"

    if result, err := resolver.Query("www.baidu.com", true); err != nil {
        t.Error(err)
    } else {
        fmt.Println(result)
    }
}
