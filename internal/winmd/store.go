package winmd

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/microsoft/go-winmd"
)

// ClassNotFoundError is returned when a class is not found.
type ClassNotFoundError struct {
	Class string
}

func (e *ClassNotFoundError) Error() string {
	return fmt.Sprintf("class %s was not found", e.Class)
}

// Store holds the windows metadata contexts. It can be used to get the metadata across multiple files.
type Store struct {
	contexts map[string]*winmd.Metadata
	logger   log.Logger
}

// NewStore loads all windows metadata files and returns a new Store.
func NewStore(logger log.Logger) (*Store, error) {
	contexts := make(map[string]*winmd.Metadata)

	winmdFiles, err := allFiles()
	if err != nil {
		return nil, err
	}

	// parse and store all files in memory
	for _, f := range winmdFiles {
		winmdCtx, err := parseWinMDFile(f.Name())
		if err != nil {
			return nil, err
		}
		contexts[f.Name()] = winmdCtx
	}

	return &Store{
		contexts: contexts,
		logger:   logger,
	}, nil
}

func parseWinMDFile(path string) (*winmd.Metadata, error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return winmd.New(f)
}

// TypeDefByName returns a type definition that matches the given name.
func (mds *Store) TypeDefByName(class string) (*TypeDef, error) {
	// the type can belong to any of the contexts
	for _, ctx := range mds.contexts {
		if td := mds.typeDefByNameAndCtx(class, ctx); td != nil {
			return td, nil // return the first match
		}
	}
	return nil, &ClassNotFoundError{Class: class}
}

func (mds *Store) typeDefByNameAndCtx(class string, ctx *winmd.Metadata) *TypeDef {
	typeDefTable := ctx.Tables.TypeDef
	for i := winmd.Index(0); i < winmd.Index(typeDefTable.Len); i++ {
		typeDef, err := typeDefTable.Record(i)
		if err != nil {
			continue // keep searching instead of failing
		}

		if typeDef.Namespace.String()+"."+typeDef.Name.String() == class {
			return &TypeDef{
				TypeDef:    typeDef,
				HasContext: HasContext{ctx},
				logger:     mds.logger,
			}
		}
	}

	return nil
}
