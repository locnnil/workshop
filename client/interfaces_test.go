package client_test

import (
	"fmt"
	"testing"

	"github.com/canonical/workshop/client"
)

func FuzzRemount(f *testing.F) {
	testcases := []string{"albert/go:plug", "albert-test/go-sdk:plug7", "albert-test/go-sdk_:a-plug", "work_shop/go-sdk_:a-plug"}
	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, a string) {
		ref, err := client.ParsePlugRef(a)
		if err == nil {
			if fmt.Sprintf("%s/%s:%s", ref.Workshop, ref.Sdk, ref.Name) != a {
				t.Errorf("plug %s cannot be reverted to a PlugRef after parsing", a)
			}
		}

		if err != nil {
			if err.Error() != fmt.Sprintf("cannot remount: unknown plug reference %s", a) {
				t.Errorf("unknown error returned: %v", err)
			}
		}
	})
}
