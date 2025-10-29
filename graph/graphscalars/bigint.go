package graphscalars

import (
	"fmt"
	"io"
	"strconv"
)

type BigInt int64

func (BigInt) ImplementsGraphQLType(name string) bool { return name == "BigInt" }

func (b *BigInt) UnmarshalGraphQL(v interface{}) error {
	switch t := v.(type) {
	case int:
		*b = BigInt(int64(t))
	case int64:
		*b = BigInt(t)
	case float64:
		*b = BigInt(int64(t))
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil { return err }
		*b = BigInt(i)
	default:
		return fmt.Errorf("invalid BigInt: %T", v)
	}
	return nil
}

func (b BigInt) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.FormatInt(int64(b), 10))
}
