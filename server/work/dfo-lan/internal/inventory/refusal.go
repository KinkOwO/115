package inventory

import (
	"errors"
	"fmt"
)

// RefusalKind describes a business failure independently of its log message.
// Each command maps the same kind to its own native client error code.
type RefusalKind byte

const (
	RefusalGeneric RefusalKind = iota
	RefusalGold
	RefusalMaterials
	RefusalLimit
	RefusalEquipment
	RefusalUnsupported
	RefusalItems
)

type refusal struct {
	kind    RefusalKind
	message string
}

func (e *refusal) Error() string { return e.message }

func Refuse(kind RefusalKind, format string, args ...any) error {
	return &refusal{kind: kind, message: fmt.Sprintf(format, args...)}
}

func RefusalOf(err error) RefusalKind {
	var e *refusal
	if errors.As(err, &e) {
		return e.kind
	}
	return RefusalGeneric
}
