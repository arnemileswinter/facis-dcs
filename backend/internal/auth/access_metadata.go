package auth

import (
	"context"
	"errors"
	"reflect"

	"digital-contracting-service/internal/auth/db"

	goa "goa.design/goa/v3/pkg"
)

type accessMetadata struct {
	Scope, Justification string
	DID                  *string
}
type accessMetadataKey struct{}

// AccessMetadataMiddleware records decoded metadata and the final access decision.
func AccessMetadataMiddleware(next goa.Endpoint) goa.Endpoint {
	return func(ctx context.Context, request any) (any, error) {
		metadata := accessMetadata{}
		value := reflect.ValueOf(request)
		if value.Kind() == reflect.Pointer && !value.IsNil() {
			value = value.Elem()
		}
		if value.IsValid() && value.Kind() == reflect.Struct {
			metadata.Scope = metadataString(value, "Scope")
			metadata.Justification = metadataString(value, "Justification")
			did := metadataString(value, "Did")
			if did == "" {
				did = metadataString(value, "ResourceID")
			}
			if did != "" {
				metadata.DID = &did
			}
		}
		pending := &accessFinalizer{}
		ctx = context.WithValue(ctx, accessMetadataKey{}, metadata)
		ctx = context.WithValue(ctx, accessFinalizerKey{}, pending)
		result, err := next(ctx, request)
		if pending.record != nil {
			pending.record(err)
		}
		return result, err
	}
}

func accessMetadataFromContext(ctx context.Context) accessMetadata {
	metadata, _ := ctx.Value(accessMetadataKey{}).(accessMetadata)
	return metadata
}

type accessFinalizerKey struct{}
type accessFinalizer struct{ record func(error) }

func accessOutcome(err error) string {
	var named interface{ ErrorName() string }
	if errors.As(err, &named) && (named.ErrorName() == "forbidden" || named.ErrorName() == "unauthorized") {
		return db.AccessOutcomeDenied
	}
	return db.AccessOutcomeSuccess
}

func metadataString(value reflect.Value, name string) string {
	field := value.FieldByName(name)
	if field.IsValid() && field.Kind() == reflect.Pointer && !field.IsNil() {
		field = field.Elem()
	}
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}
