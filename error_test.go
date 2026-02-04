package bhttp_test

import (
	"testing"

	"github.com/advdv/bhttp"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestErrorCode(t *testing.T) {
	err1 := bhttp.NewError(bhttp.CodeBadRequest, errors.New("foo"))
	require.Equal(t, bhttp.Code(400), err1.Code())
	require.Equal(t, bhttp.CodeBadRequest, bhttp.CodeOf(err1))
	require.Equal(t, "Bad Request: foo", err1.Error())

	require.Equal(t, bhttp.CodeUnknown, bhttp.CodeOf(errors.New("bar")))
	require.Equal(t, "Unknown: rab", bhttp.NewError(900, errors.New("rab")).Error())
}
