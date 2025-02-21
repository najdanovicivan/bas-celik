package card

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

type BER struct {
	tag       uint32
	primitive bool
	data      []byte
	children  []BER
}

func ParseBER(data []byte) (*BER, error) {
	primitive, constructed, err := parseBERLayer(data)

	if err != nil {
		return nil, err
	}

	ber := BER{
		tag:       0,
		primitive: false,
		data:      nil,
		children:  []BER{},
	}

	for t, v := range primitive {
		val := BER{
			tag:       t,
			primitive: true,
			data:      v,
			children:  nil,
		}

		err = ber.add(val)
		if err != nil {
			return nil, fmt.Errorf("adding primitive value: %w", err)
		}
	}

	for t, v := range constructed {
		subBer, err := ParseBER(v)
		if err != nil {
			return nil, err
		}
		val := BER{
			tag:       t,
			primitive: false,
			data:      nil,
			children:  subBer.children,
		}

		err = ber.add(val)
		if err != nil {
			return nil, fmt.Errorf("adding primitive value: %w", err)
		}
	}

	return &ber, nil

}

func (tree BER) access(address ...uint32) ([]byte, error) {
	if len(address) == 0 {
		return tree.data, nil
	} else {
		var found *BER = nil
		for i := range tree.children {
			if tree.children[i].tag == address[0] {
				found = &tree.children[i]
				break
			}
		}
		if found != nil {
			return found.access(address[1:]...)
		} else {
			return nil, errors.New("tag not found")
		}
	}
}

// Insert a new node into BER tree
func (into *BER) add(new BER) error {
	if into.primitive {
		return errors.New("can't add a value into primitive value")
	}

	var targetField *BER

	alreadyExists := false
	for i := range into.children {
		if into.children[i].tag == new.tag {
			alreadyExists = true
			targetField = &into.children[i]
			break
		}
	}

	if !alreadyExists {
		into.children = append(into.children, new)
		return nil
	} else {
		if targetField.primitive == new.primitive {
			if targetField.primitive {
				*targetField = new
			} else {
				for i := range new.children {
					err := targetField.add(new.children[i])
					if err != nil {
						return err
					}
				}
			}
		} else {
			return errors.New("types don't match")
		}
	}

	return nil
}

func (into *BER) merge(new BER) error {
	if into.tag != new.tag {
		return errors.New("tags don't match")
	}

	for _, c := range new.children {
		if err := into.add(c); err != nil {
			return err
		}
	}

	return nil
}

// Parses one level of BER-TLV encoded data
// according to ISO/IEC 7816-4 (2005)
// Returns map of primitive and constructed fields
func parseBERLayer(data []byte) (map[uint32][]byte, map[uint32][]byte, error) {
	primF := make(map[uint32][]byte)
	consF := make(map[uint32][]byte)
	var primitive bool
	offset := uint32(0)

	for {
		tag := uint32(data[offset])

		if tag&0x20 != 0 {
			primitive = false
		} else {
			primitive = true
		}

		if 0x1F&tag != 0x1F {
			offset += 1
		} else {
			if data[offset+1]&0x80 == 0x00 {
				tag = uint32(data[offset])<<8 + uint32(data[offset+1])
				offset += 2
			} else {
				tag = uint32(data[offset])<<16 +
					uint32(data[offset+1])<<8 +
					uint32(data[offset+2])
				offset += 3
			}
		}

		length, offsetDelta, err := parseBerLength(data[offset:])
		if err != nil {
			return nil, nil, errors.New("invalid length")
		}

		offset += offsetDelta
		value := data[offset : offset+length]

		if primitive {
			primF[tag] = value
		} else {
			consF[tag] = value
		}

		offset += length

		if offset == uint32(len(data)) {
			break
		} else if offset > uint32(len(data)) {
			return nil, nil, errors.New("invalid length")
		}
	}

	return primF, consF, nil
}

func (tree *BER) assignFrom(target *string, address ...uint32) {
	bytes, err := tree.access(address...)
	if err == nil {
		*target = string(bytes)
	}
}

func (tree *BER) levels() []string {
	if tree.primitive {
		return []string{fmt.Sprintf("%X: %s", tree.tag, string(tree.data))}
	} else {
		strings := []string{fmt.Sprint(tree.tag) + ":"}
		for _, child := range tree.children {
			childrenStrings := child.levels()
			for i := range childrenStrings {
				childrenStrings[i] = "  " + childrenStrings[i]
			}
			strings = append(strings, childrenStrings...)
		}

		return strings
	}
}

func (tree BER) String() string {
	return strings.Join(tree.levels(), "\n")
}

func parseBerLength(data []byte) (uint32, uint32, error) {
	length := uint32(data[0])
	offset := uint32(0)
	if length < 0x80 {
		offset = 1
	} else if length == 0x81 {
		length = uint32(data[1])
		offset = 2
	} else if length == 0x82 {
		length = uint32(binary.BigEndian.Uint16(data[1:]))
		offset = 3
	} else if length == 0x83 {
		length = 0x00FFFFFF & binary.BigEndian.Uint32(data)
		offset = 4
	} else if length == 0x84 {
		length = binary.BigEndian.Uint32(data[1:])
		offset = 5
	} else {
		return 0, 0, errors.New("invalid length")
	}

	return length, uint32(offset), nil
}
