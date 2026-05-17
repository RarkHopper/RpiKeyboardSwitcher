package input

import "errors"

var (
	errEmptyReportDescriptor     = errors.New("HID report descriptor is empty")
	errTruncatedLongItem         = errors.New("HID long item is truncated")
	errTruncatedLongItemPayload  = errors.New("HID long item payload is truncated")
	errTruncatedShortItemPayload = errors.New("HID short item payload is truncated")
	errInvalidReportIDSize       = errors.New("HID report ID item must be one byte")
	errZeroReportID              = errors.New("HID report ID must not be zero")
	errNoInputReports            = errors.New("HID report descriptor has no input reports")
)

type Descriptor struct {
	ReportMap       []byte
	InputReportIDs  []byte
	OutputReportIDs []byte
	UsesReportID    bool
}

type Report struct {
	ID   byte
	Data []byte
}

func ParseDescriptor(reportMap []byte) (Descriptor, error) {
	if len(reportMap) == 0 {
		return Descriptor{}, errEmptyReportDescriptor
	}

	descriptor := Descriptor{
		ReportMap: append([]byte(nil), reportMap...),
	}
	seenInput := map[byte]bool{}
	seenOutput := map[byte]bool{}
	reportID := byte(0x00)

	for index := 0; index < len(reportMap); {
		prefix := reportMap[index]
		index++
		if prefix == 0xfe {
			if index+2 > len(reportMap) {
				return Descriptor{}, errTruncatedLongItem
			}
			size := int(reportMap[index])
			index += 2
			if index+size > len(reportMap) {
				return Descriptor{}, errTruncatedLongItemPayload
			}
			index += size
			continue
		}

		size := int(prefix & 0x03)
		if size == 3 {
			size = 4
		}
		itemType := (prefix >> 2) & 0x03
		tag := (prefix >> 4) & 0x0f
		if index+size > len(reportMap) {
			return Descriptor{}, errTruncatedShortItemPayload
		}
		value := reportMap[index : index+size]
		index += size

		if itemType == 1 && tag == 8 {
			if len(value) != 1 {
				return Descriptor{}, errInvalidReportIDSize
			}
			if value[0] == 0x00 {
				return Descriptor{}, errZeroReportID
			}
			reportID = value[0]
			descriptor.UsesReportID = true
			continue
		}
		if itemType == 0 && tag == 8 && !seenInput[reportID] {
			descriptor.InputReportIDs = append(descriptor.InputReportIDs, reportID)
			seenInput[reportID] = true
		}
		if itemType == 0 && tag == 9 && !seenOutput[reportID] {
			descriptor.OutputReportIDs = append(descriptor.OutputReportIDs, reportID)
			seenOutput[reportID] = true
		}
	}

	if len(descriptor.InputReportIDs) == 0 {
		return Descriptor{}, errNoInputReports
	}

	return descriptor, nil
}

func (descriptor Descriptor) Report(raw []byte) (Report, bool) {
	if len(raw) == 0 {
		return Report{}, false
	}

	if !descriptor.UsesReportID {
		return Report{
			ID:   0x00,
			Data: append([]byte(nil), raw...),
		}, true
	}

	id := raw[0]
	if !descriptor.hasInputReportID(id) {
		return Report{}, false
	}

	return Report{
		ID:   id,
		Data: append([]byte(nil), raw[1:]...),
	}, true
}

func (descriptor Descriptor) hasInputReportID(id byte) bool {
	for _, candidate := range descriptor.InputReportIDs {
		if candidate == id {
			return true
		}
	}

	return false
}
