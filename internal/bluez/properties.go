package bluez

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

type propertiesObject struct {
	getProperties func(string) (map[string]dbus.Variant, bool)
}

func exportProperties(conn *dbus.Conn, path dbus.ObjectPath, getProperties func(string) (map[string]dbus.Variant, bool)) error {
	return conn.Export(propertiesObject{getProperties: getProperties}, path, PropertiesInterface)
}

func (object propertiesObject) Get(interfaceName string, propertyName string) (dbus.Variant, *dbus.Error) {
	properties, ok := object.getProperties(interfaceName)
	if !ok {
		return dbus.Variant{}, unknownInterface(interfaceName)
	}

	value, ok := properties[propertyName]
	if !ok {
		return dbus.Variant{}, invalidArguments("unknown property: " + propertyName)
	}

	return value, nil
}

func (object propertiesObject) GetAll(interfaceName string) (map[string]dbus.Variant, *dbus.Error) {
	properties, ok := object.getProperties(interfaceName)
	if !ok {
		return nil, unknownInterface(interfaceName)
	}

	return properties, nil
}

func (object propertiesObject) Set(interfaceName string, propertyName string, _ dbus.Variant) *dbus.Error {
	if _, ok := object.getProperties(interfaceName); !ok {
		return unknownInterface(interfaceName)
	}

	return invalidArguments(fmt.Sprintf("property is read-only: %s", propertyName))
}

func unknownInterface(interfaceName string) *dbus.Error {
	err := dbus.MakeUnknownInterfaceError(interfaceName)
	return &err
}

func invalidArguments(message string) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []any{message})
}
