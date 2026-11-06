package generic

import (
	"context"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/thrift/base"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

type decoratedTestReader struct {
	reader thrift.MessageReader
	seen   *int
}

func (r decoratedTestReader) Read(ctx context.Context, method string, response bool, n int, in bufiox.Reader) (interface{}, error) {
	*r.seen++
	return r.reader.Read(ctx, method, response, n, in)
}

func TestCodecDecoratorRetainsConfiguredSnapshotAndMessageTypes(t *testing.T) {
	provider, err := NewThriftContentProvider(`struct Request {1:string Body} service Test {Request Echo(1:Request req)}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	reader := thrift.NewStructReaderWriter(<-provider.Provide())
	var raw []byte
	out := bufiox.NewBytesWriter(&raw)
	if err = reader.Write(context.Background(), out, map[string]interface{}{"Body": "value"}, "Echo", true, nil); err != nil {
		t.Fatal(err)
	}
	if err = out.Flush(); err != nil {
		t.Fatal(err)
	}
	args := &Args{}
	args.SetCodec(reader)
	called := 0
	ctx := WithCodecDecorator(context.Background(), func(actual interface{}, cached *base.Base) interface{} {
		if actual != reader || cached != nil {
			t.Fatalf("snapshot/cache changed: %T %v", actual, cached)
		}
		return decoratedTestReader{actual.(thrift.MessageReader), &called}
	})
	if err = args.Read(ctx, "Echo", len(raw), bufiox.NewBytesReader(raw)); err != nil {
		t.Fatal(err)
	}
	if called != 1 || !reflect.DeepEqual(args.Request, map[string]interface{}{"Body": "value"}) {
		t.Fatal(called, args.Request)
	}
	// A context decorator does not replace the codec used by later calls.
	if err = args.Read(context.Background(), "Echo", len(raw), bufiox.NewBytesReader(raw)); err != nil || called != 1 {
		t.Fatal(called, err)
	}
	if !DecorateMessageCodec(args, func(actual interface{}, cached *base.Base) interface{} {
		if actual != reader || cached != nil {
			t.Fatal("inspection allocated Base")
		}
		return actual
	}) {
		t.Fatal("Args not supported")
	}
	if DecorateMessageCodec((*Args)(nil), func(v interface{}, _ *base.Base) interface{} { return v }) {
		t.Fatal("accepted nil Args")
	}
	if DecorateMessageCodec(struct{}{}, func(v interface{}, _ *base.Base) interface{} { return v }) {
		t.Fatal("accepted non-generic message")
	}
}

func TestCodecDecoratorFollowsDescriptorProviderUpdates(t *testing.T) {
	const initial = `struct Request {1:string Body} service Test {Request Echo(1:Request req)}`
	const updated = `struct Request {1:string Body,3:string Version} service Test {Request Echo(1:Request req),Request Added(1:Request req)}`
	provider, err := NewThriftContentProvider(initial, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := MapThriftGeneric(provider)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	old := g.GenericMethod()(context.Background(), "Echo").NewArgs().(*Args)
	if err = provider.UpdateIDL(updated, nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for g.GenericMethod()(context.Background(), "Added") == nil {
		if time.Now().After(deadline) {
			t.Fatal("descriptor update not observed")
		}
		time.Sleep(time.Millisecond)
	}
	fresh := g.GenericMethod()(context.Background(), "Echo").NewArgs().(*Args)
	raw := []byte{12, 0, 1, 11, 0, 1, 0, 0, 0, 1, 'x', 11, 0, 3, 0, 0, 0, 2, 'v', '2', 0, 0}
	for index, args := range []*Args{old, fresh} {
		seen := false
		ctx := WithCodecDecorator(context.Background(), func(actual interface{}, _ *base.Base) interface{} {
			svc := actual.(interface {
				ServiceDescriptor() *descriptor.ServiceDescriptor
			}).ServiceDescriptor()
			field := svc.Functions["Echo"].Request.Struct.FieldsByID[1].Type.Struct.FieldsByID[3]
			if (field != nil) != (index == 1) {
				t.Fatal("decorator used a different descriptor snapshot")
			}
			seen = true
			return actual
		})
		if err = args.Read(ctx, "Echo", len(raw), bufiox.NewBytesReader(raw)); err != nil {
			t.Fatal(err)
		}
		if !seen {
			t.Fatal("decorator not invoked")
		}
		if (args.Request.(map[string]interface{})["Version"] == "v2") != (index == 1) {
			t.Fatal("message snapshot behavior changed")
		}
	}
}
