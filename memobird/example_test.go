package memobird_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/ruhuang2001/memobird-go/textrender"
)

// newExampleServer stands in for the Memobird Web API so the examples run
// without credentials or a real printer.
func newExampleServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/home/setuserbind", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"showapi_userid":123}`))
	})
	mux.HandleFunc("/home/getSignalBase64Pic", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"result":"bitmap"}`))
	})
	mux.HandleFunc("/home/printpaper", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"result":1,"smartGuid":"g1","printcontentid":42}`))
	})
	mux.HandleFunc("/home/getprintstatus", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"printflag":1,"printcontentid":42}`))
	})
	return httptest.NewServer(mux)
}

func newExampleClient(serverURL string) *memobird.Client {
	client, err := memobird.NewClient(memobird.Config{
		AccessKey: "example-ak",
		DeviceID:  "example-device",
		BaseURL:   serverURL,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	return client
}

// Bind a user once, then print content as that user. The user ID stays on the
// client for later print calls.
func Example() {
	server := newExampleServer()
	defer server.Close()

	ctx := context.Background()
	client := newExampleClient(server.URL)

	bind, err := client.BindAndRemember(ctx, "user-identifying-string")
	if err != nil {
		panic(err)
	}
	fmt.Println("bound user:", bind.UserID)

	// PrintText renders the text to an image and prints it. Every print goes
	// through an image: the vendor cannot decode Chinese text.
	if _, err := client.PrintText(ctx, "Hello, Memobird!", textrender.Options{}); err != nil {
		panic(err)
	}
	fmt.Println("print submitted")
	// Output:
	// bound user: 123
	// print submitted
}

// Submit a print job, then poll until the printer reports it as printed.
func ExampleClient_WaitForPrint() {
	server := newExampleServer()
	defer server.Close()

	ctx := context.Background()
	client := newExampleClient(server.URL)
	client.SetUserID(123)

	printed, err := client.PrintText(ctx, "Hello", textrender.Options{})
	if err != nil {
		panic(err)
	}

	status, err := client.WaitForPrint(ctx, printed.PrintContentID, memobird.WaitOptions{
		Interval: 500 * time.Millisecond,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("printed:", status.IsPrinted())
	// Output:
	// printed: true
}

// Detect a rejection from the remote API by checking for *APIError.
func ExampleAPIError() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"showapi_res_code":0,"showapi_res_error":"invalid access key"}`))
	}))
	defer server.Close()

	client, err := memobird.NewClient(memobird.Config{
		AccessKey: "bad-ak",
		DeviceID:  "device",
		BaseURL:   server.URL,
	})
	if err != nil {
		panic(err)
	}

	_, err = client.BindUser(context.Background(), "user")
	var apiErr *memobird.APIError
	if errors.As(err, &apiErr) {
		fmt.Println("action:", apiErr.Action)
		fmt.Println("code:", apiErr.Code)
		fmt.Println("message:", apiErr.Message)
	}
	// Output:
	// action: bind
	// code: 0
	// message: invalid access key
}

// PrintText requires a bound user; the sentinel error is testable with errors.Is.
func ExampleErrUserIDNotConfigured() {
	client, err := memobird.NewClient(memobird.Config{
		AccessKey: "ak",
		DeviceID:  "device",
		BaseURL:   "http://127.0.0.1:1",
	})
	if err != nil {
		panic(err)
	}

	_, err = client.PrintText(context.Background(), "Hello", textrender.Options{})
	fmt.Println(errors.Is(err, memobird.ErrUserIDNotConfigured))
	// Output:
	// true
}
