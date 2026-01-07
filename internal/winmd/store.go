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

// Store holds the windows metadata. It can be used to get the metadata across multiple files.
type Store struct {
	metadatas map[string]*winmd.Metadata
	logger    log.Logger
}

// NewStore loads all windows metadata files and returns a new Store.
func NewStore(logger log.Logger) (*Store, error) {
	metadatas := make(map[string]*winmd.Metadata)

	winmdFiles, err := allFiles()
	if err != nil {
		return nil, err
	}

	// parse and store all files in memory
	for _, f := range winmdFiles {
		winmdMetadata, err := parseWinMDFile(f.Name())
		if err != nil {
			return nil, err
		}
		metadatas[f.Name()] = winmdMetadata
	}

	return &Store{
		metadatas: metadatas,
		logger:    logger,
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
	// the type can belong to any of the metadatas
	for _, metadata := range mds.metadatas {
		if td := mds.typeDefByNameAndMetadata(class, metadata); td != nil {
			return td, nil // return the first match
		}
	}
	return nil, &ClassNotFoundError{Class: class}
}

func (mds *Store) typeDefByNameAndMetadata(class string, metadata *winmd.Metadata) *TypeDef {
	// Iterate through TypeDef table
	for i := winmd.Index(1); i <= winmd.Index(metadata.Tables.TypeDef.Len); i++ {
		typeDef, err := metadata.Tables.TypeDef.Record(i)
		if err != nil {
			continue // keep searching instead of failing
		}

		typeNamespace, err := metadata.Strings.String(typeDef.Namespace.Start)
		if err != nil {
			continue
		}
		typeName, err := metadata.Strings.String(typeDef.Name.Start)
		if err != nil {
			continue
		}

		if typeNamespace.String()+"."+typeName.String() == class {
			return &TypeDef{
				TypeDef:     typeDef,
				HasMetadata: HasMetadata{metadata},
				logger:      mds.logger,
			}
		}
	}

	return nil
}
