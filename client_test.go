package httpadapter_test

import (
	"fmt"
	"testing"

	"github.com/powerpuffpenguin/httpadapter"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	s := newServer(t)
	defer s.CloseAndWait()

	client := httpadapter.NewClient(Addr)
	c, e := client.Dial()
	if !assert.Nil(t, e) {
		t.FailNow()
	}
	// defer c.Close()
	fmt.Println(c)

}
