// conf marshalling
package models

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"fmt"
	"log"
	"reflect"
)

const (
	nameTag       = "toml"
	singleLineTag = "singleline"
)

type Metadata struct {
	name            string
	arrayKind       bool
	encodeBase64    bool
	singleArrayLine bool
	structKind      bool
	anonField       bool
}

func getMetaData(rsf reflect.StructField) (meta Metadata) {
	rsfT := rsf.Type
	meta.name = rsf.Tag.Get(nameTag)

	if meta.name == "" {
		meta.name = rsf.Name
	}
	// log.Println("name:", meta.name, "kind:", rsfT.Kind())

	if rsfT.Kind() == reflect.Array || rsfT.Kind() == reflect.Slice {
		meta.arrayKind = true

		if rsfT.Elem().Kind() == reflect.String && rsf.Tag.Get(singleLineTag) == "true" {
			meta.singleArrayLine = true
		}

		if rsfT.Elem().Kind() == reflect.Uint8 {
			meta.encodeBase64 = true
		}
	} else if rsfT.Kind() == reflect.Struct {
		meta.structKind = true
		meta.anonField = rsf.Anonymous
		if !(rsfT.Implements(reflect.TypeFor[encoding.TextMarshaler]())) {
			log.Panicln("struct needs to implement TextMarshaler")
		}
	}

	return
}

func writeBuffer(buffer *bytes.Buffer, buf []byte) error {
	if _, err := buffer.Write(buf); err != nil {
		return err
	}
	return nil
}

func writeBufferString(buffer *bytes.Buffer, str string) error {
	if _, err := buffer.WriteString(str); err != nil {
		return err
	}
	return nil
}

func handlePrimitve(buffer *bytes.Buffer, rv reflect.Value, meta Metadata) error {
	switch rv.Kind() {
	case reflect.String:
		if rv.String() == "" {
			return nil
		}
		return writeBufferString(buffer, fmt.Sprintf("%s = %s\n", meta.name, rv.String()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// skipping here if rv.Int() == 0 since fwMark as 0 seems to create problems in android-wireguard
		if rv.Int() == 0 {
			return nil
		}
		return writeBufferString(buffer, fmt.Sprintf("%s = %d\n", meta.name, rv.Int()))
	default:
		log.Panicln("unkown primitive type")
	}
	return nil
}

func handleStruct(buffer *bytes.Buffer, rv reflect.Value, meta Metadata) error {
	if !meta.anonField {
		if err := writeBufferString(buffer, fmt.Sprintf("[%s]\n", meta.name)); err != nil {
			return err
		}
	}
	if buf, err := rv.Interface().(encoding.TextMarshaler).MarshalText(); err != nil {
		return err
	} else if err := writeBuffer(buffer, buf); err != nil {
		return err
	}

	if !meta.anonField {
		return writeBufferString(buffer, "\n")
	}
	return nil
}

func handleSingleArrayString(buffer *bytes.Buffer, rv reflect.Value, meta Metadata) error {
	if rv.Len() == 0 {
		return nil
	}

	if err := writeBufferString(buffer, fmt.Sprintf("%s = ", meta.name)); err != nil {
		return err
	}

	for i := 0; i < rv.Len(); i++ {
		str := rv.Index(i).String()

		if i == 0 {
			if err := writeBufferString(buffer, str); err != nil {
				return err
			}
		} else {
			if err := writeBufferString(buffer, fmt.Sprintf(", %s", str)); err != nil {
				return err
			}
		}
	}
	return writeBufferString(buffer, "\n")
}

func handleByteArray(buffer *bytes.Buffer, rv reflect.Value, meta Metadata) error {
	// can't convert to slice if unaddressable array... need to loop
	var buf []byte
	for i := 0; i < rv.Len(); i++ {
		buf = append(buf, rv.Index(i).Interface().(byte))
	}
	base64Encoded := base64.StdEncoding.EncodeToString(buf)
	return writeBufferString(buffer, fmt.Sprintf("%s = %s\n", meta.name, base64Encoded))
}

func handleArray(buffer *bytes.Buffer, rv reflect.Value, meta Metadata) error {
	if meta.encodeBase64 {
		return handleByteArray(buffer, rv, meta)
	} else if meta.singleArrayLine {
		return handleSingleArrayString(buffer, rv, meta)
	} else {
		arrElemT := rv.Type().Elem()

		if arrElemT.Kind() == reflect.Struct {
			// struct type
			if !arrElemT.Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
				log.Panicln("struct needs to implement TextMarshaler")
			}

			for i := 0; i < rv.Len(); i++ {
				if err := handleStruct(buffer, rv.Index(i), meta); err != nil {
					return err
				}
			}
		} else {
			// can't convert to slice if unaddressable array
			for i := 0; i < rv.Len(); i++ {
				if err := handlePrimitve(buffer, rv.Index(i), meta); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func confMarshallStruct(v any) (text []byte, err error) {
	rv := reflect.ValueOf(v)
	rvT := rv.Type()

	if rv.Kind() == reflect.Struct {
		var buffer bytes.Buffer

		for i := 0; i < rv.NumField(); i++ {
			val := rv.Field(i)
			meta := getMetaData(rvT.Field(i))
			if meta.arrayKind {
				if err := handleArray(&buffer, val, meta); err != nil {
					return nil, err
				}
			} else if meta.structKind {
				if err := handleStruct(&buffer, val, meta); err != nil {
					return nil, err
				}
			} else {
				if err := handlePrimitve(&buffer, val, meta); err != nil {
					return nil, err
				}
			}
		}
		text = buffer.Bytes()
	} else {
		log.Panicln("only called on struct")
	}
	return

}

// --- TextMarshaler implemented by types ---
func (v Credentials) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v Peer) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v Interface) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v ServerInterface) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v Config) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v ClientConfig) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}

func (v ServerConfig) MarshalText() (text []byte, err error) {
	return confMarshallStruct(v)
}
