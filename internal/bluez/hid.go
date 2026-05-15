package bluez

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	AdapterInterface              = "org.bluez.Adapter1"
	AgentInterface                = "org.bluez.Agent1"
	AgentManagerInterface         = "org.bluez.AgentManager1"
	GATTManagerInterface          = "org.bluez.GattManager1"
	GATTServiceInterface          = "org.bluez.GattService1"
	GATTCharacteristicInterface   = "org.bluez.GattCharacteristic1"
	GATTDescriptorInterface       = "org.bluez.GattDescriptor1"
	DeviceInterface               = "org.bluez.Device1"
	LEAdvertisementInterface      = "org.bluez.LEAdvertisement1"
	LEAdvertisingManagerInterface = "org.bluez.LEAdvertisingManager1"
	ObjectManagerInterface        = "org.freedesktop.DBus.ObjectManager"
	PropertiesInterface           = "org.freedesktop.DBus.Properties"

	// Bluetooth SIG Assigned Numbers: https://www.bluetooth.com/specifications/assigned-numbers/
	HIDServiceUUID               = "00001812-0000-1000-8000-00805f9b34fb"
	HIDInformationUUID           = "00002a4a-0000-1000-8000-00805f9b34fb"
	ReportMapUUID                = "00002a4b-0000-1000-8000-00805f9b34fb"
	HIDControlPointUUID          = "00002a4c-0000-1000-8000-00805f9b34fb"
	ReportUUID                   = "00002a4d-0000-1000-8000-00805f9b34fb"
	ProtocolModeUUID             = "00002a4e-0000-1000-8000-00805f9b34fb"
	BootKeyboardInputReportUUID  = "00002a22-0000-1000-8000-00805f9b34fb"
	BootKeyboardOutputReportUUID = "00002a32-0000-1000-8000-00805f9b34fb"
	ReportReferenceUUID          = "00002908-0000-1000-8000-00805f9b34fb"

	AppPath           dbus.ObjectPath = "/com/rarkhopper/RpiKeyboardSwitcher/hid"
	AgentPath         dbus.ObjectPath = "/com/rarkhopper/RpiKeyboardSwitcher/agent"
	AdvertisementPath dbus.ObjectPath = "/com/rarkhopper/RpiKeyboardSwitcher/advertisement0"
	ServicePath       dbus.ObjectPath = AppPath + "/service0"
	HIDInfoPath       dbus.ObjectPath = ServicePath + "/char0"
	ReportMapPath     dbus.ObjectPath = ServicePath + "/char1"
	ControlPointPath  dbus.ObjectPath = ServicePath + "/char2"
	ProtocolModePath  dbus.ObjectPath = ServicePath + "/char3"
	ReportPath        dbus.ObjectPath = ServicePath + "/char4"
	BootInputPath     dbus.ObjectPath = ServicePath + "/char5"
	BootOutputPath    dbus.ObjectPath = ServicePath + "/char6"

	KeyboardAppearance uint16 = 0x03c1
)

type emitter interface {
	Emit(path dbus.ObjectPath, name string, values ...any) error
}

type HIDApplication struct {
	mu              sync.Mutex
	service         *Service
	characteristics map[dbus.ObjectPath]*Characteristic
	descriptors     map[dbus.ObjectPath]*Descriptor
	emitter         emitter
	subscribed      chan struct{}
	subscribeOnce   sync.Once
}

type Service struct {
	path    dbus.ObjectPath
	uuid    string
	primary bool
}

type Characteristic struct {
	app          *HIDApplication
	path         dbus.ObjectPath
	uuid         string
	servicePath  dbus.ObjectPath
	flags        []string
	value        []byte
	notifying    bool
	notify       bool
	writable     bool
	protocolMode bool
}

type Descriptor struct {
	path               dbus.ObjectPath
	uuid               string
	characteristicPath dbus.ObjectPath
	flags              []string
	value              []byte
}

type HIDAdvertisement struct {
	name       string
	appearance uint16
}

type DaemonOptions struct {
	Adapter      string
	Name         string
	Appearance   uint16
	Pairable     bool
	Discoverable bool
	TestReports  [][]byte
	InputReports func(context.Context, func([]byte) error) error
	OnPeerReady  func(Peer) error
	Log          io.Writer
}

type DBusDaemon struct{}

type Peer struct {
	Name         string
	BluetoothMAC string
}

func NewHIDApplication() *HIDApplication {
	app := &HIDApplication{
		service: &Service{
			path:    ServicePath,
			uuid:    HIDServiceUUID,
			primary: true,
		},
		characteristics: make(map[dbus.ObjectPath]*Characteristic),
		descriptors:     make(map[dbus.ObjectPath]*Descriptor),
		subscribed:      make(chan struct{}),
	}

	app.addCharacteristic(HIDInfoPath, HIDInformationUUID, []string{"read"}, []byte{0x11, 0x01, 0x00, 0x02}, false, false, false)
	app.addCharacteristic(ReportMapPath, ReportMapUUID, []string{"read"}, reportMap(), false, false, false)
	app.addCharacteristic(ControlPointPath, HIDControlPointUUID, []string{"write-without-response"}, nil, false, true, false)
	app.addCharacteristic(ProtocolModePath, ProtocolModeUUID, []string{"read", "write-without-response"}, []byte{0x01}, false, true, true)
	app.addCharacteristic(ReportPath, ReportUUID, []string{"read", "notify"}, make([]byte, 8), true, false, false)
	app.addCharacteristic(BootInputPath, BootKeyboardInputReportUUID, []string{"read", "notify"}, make([]byte, 8), true, false, false)
	app.addCharacteristic(BootOutputPath, BootKeyboardOutputReportUUID, []string{"read", "write", "write-without-response"}, []byte{0x00}, false, true, false)
	app.addDescriptor(ReportPath+"/desc0", ReportReferenceUUID, ReportPath, []string{"read"}, []byte{0x00, 0x01})

	return app
}

func NewHIDAdvertisement(name string, appearance uint16) *HIDAdvertisement {
	return &HIDAdvertisement{
		name:       name,
		appearance: appearance,
	}
}

func (app *HIDApplication) SetEmitter(emitter emitter) {
	app.mu.Lock()
	defer app.mu.Unlock()

	app.emitter = emitter
}

func (app *HIDApplication) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	return app.ManagedObjects(), nil
}

func (app *HIDApplication) ManagedObjects() map[dbus.ObjectPath]map[string]map[string]dbus.Variant {
	app.mu.Lock()
	defer app.mu.Unlock()

	objects := make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant, 1+len(app.characteristics)+len(app.descriptors))
	objects[app.service.path] = map[string]map[string]dbus.Variant{
		GATTServiceInterface: app.service.properties(),
	}
	for path, characteristic := range app.characteristics {
		objects[path] = map[string]map[string]dbus.Variant{
			GATTCharacteristicInterface: characteristic.propertiesLocked(),
		}
	}
	for path, descriptor := range app.descriptors {
		objects[path] = map[string]map[string]dbus.Variant{
			GATTDescriptorInterface: descriptor.properties(),
		}
	}

	return objects
}

func (app *HIDApplication) WaitForSubscription(ctx context.Context) error {
	select {
	case <-app.subscribed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (app *HIDApplication) SendReports(reports [][]byte) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	for _, report := range reports {
		if len(report) != 8 {
			return fmt.Errorf("HID keyboard report must be 8 bytes: got %d", len(report))
		}
		if err := app.notifyInputLocked(report); err != nil {
			return err
		}
	}

	return nil
}

func (app *HIDApplication) SendReport(report []byte) error {
	return app.SendReports([][]byte{report})
}

func (app *HIDApplication) SendReportsAfterSubscription(ctx context.Context, reports [][]byte) error {
	if len(reports) == 0 {
		return nil
	}
	if err := app.WaitForSubscription(ctx); err != nil {
		return err
	}

	return app.SendReports(reports)
}

func (app *HIDApplication) Export(conn *dbus.Conn) error {
	app.SetEmitter(conn)

	if err := conn.Export(app, AppPath, ObjectManagerInterface); err != nil {
		return err
	}
	if err := conn.ExportMethodTable(map[string]any{}, ServicePath, GATTServiceInterface); err != nil {
		return err
	}
	if err := exportProperties(conn, ServicePath, app.serviceProperties); err != nil {
		return err
	}

	for path, characteristic := range app.characteristics {
		if err := conn.Export(characteristic, path, GATTCharacteristicInterface); err != nil {
			return err
		}
		if err := exportProperties(conn, path, characteristic.propertiesForInterface); err != nil {
			return err
		}
	}
	for path, descriptor := range app.descriptors {
		if err := conn.Export(descriptor, path, GATTDescriptorInterface); err != nil {
			return err
		}
		if err := exportProperties(conn, path, descriptor.propertiesForInterface); err != nil {
			return err
		}
	}

	return nil
}

func (advertisement *HIDAdvertisement) Properties() map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		"Type":         dbus.MakeVariant("peripheral"),
		"ServiceUUIDs": dbus.MakeVariant([]string{HIDServiceUUID}),
		"LocalName":    dbus.MakeVariant(advertisement.name),
		"Appearance":   dbus.MakeVariant(advertisement.appearance),
	}
	return props
}

func (advertisement *HIDAdvertisement) Release() *dbus.Error {
	return nil
}

func (advertisement *HIDAdvertisement) Export(conn *dbus.Conn) error {
	if err := conn.Export(advertisement, AdvertisementPath, LEAdvertisementInterface); err != nil {
		return err
	}

	return exportProperties(conn, AdvertisementPath, func(interfaceName string) (map[string]dbus.Variant, bool) {
		if interfaceName != LEAdvertisementInterface {
			return nil, false
		}

		return advertisement.Properties(), true
	})
}

func (DBusDaemon) Run(ctx context.Context, options DaemonOptions) error {
	logf(options.Log, "Serving BLE HID keyboard as %s on %s\n", options.Name, options.Adapter)

	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()
	if err := conn.Auth(nil); err != nil {
		return err
	}
	if err := conn.Hello(); err != nil {
		return err
	}

	app := NewHIDApplication()
	advertisement := NewHIDAdvertisement(options.Name, options.Appearance)
	agent := NewAgent(options.Log)

	if err := app.Export(conn); err != nil {
		return err
	}
	if err := advertisement.Export(conn); err != nil {
		return err
	}
	if err := agent.Export(conn); err != nil {
		return err
	}

	adapterPath := dbus.ObjectPath("/org/bluez/" + options.Adapter)
	adapter := conn.Object("org.bluez", adapterPath)
	if err := adapter.SetProperty(AdapterInterface+".Powered", dbus.MakeVariant(true)); err != nil {
		return err
	}
	if err := adapter.SetProperty(AdapterInterface+".Pairable", dbus.MakeVariant(options.Pairable)); err != nil {
		return err
	}
	if err := adapter.SetProperty(AdapterInterface+".Discoverable", dbus.MakeVariant(options.Discoverable)); err != nil {
		return err
	}

	bluez := conn.Object("org.bluez", "/org/bluez")
	if err := bluez.Call(AgentManagerInterface+".RegisterAgent", 0, AgentPath, "KeyboardDisplay").Err; err != nil {
		return err
	}
	defer func() {
		_ = bluez.Call(AgentManagerInterface+".UnregisterAgent", 0, AgentPath).Err
	}()
	if err := bluez.Call(AgentManagerInterface+".RequestDefaultAgent", 0, AgentPath).Err; err != nil {
		return err
	}

	if err := adapter.Call(GATTManagerInterface+".RegisterApplication", 0, AppPath, map[string]dbus.Variant{}).Err; err != nil {
		return err
	}
	defer func() {
		_ = adapter.Call(GATTManagerInterface+".UnregisterApplication", 0, AppPath).Err
	}()
	if err := adapter.Call(LEAdvertisingManagerInterface+".RegisterAdvertisement", 0, AdvertisementPath, map[string]dbus.Variant{}).Err; err != nil {
		return err
	}
	defer func() {
		_ = adapter.Call(LEAdvertisingManagerInterface+".UnregisterAdvertisement", 0, AdvertisementPath).Err
	}()

	readyErrors := make(chan error, 1)
	if len(options.TestReports) > 0 || options.OnPeerReady != nil || options.InputReports != nil {
		go func() {
			if err := app.WaitForSubscription(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					readyErrors <- err
				}
				return
			}
			if options.OnPeerReady != nil {
				peer, err := ConnectedPeer(ctx, conn, options.Adapter)
				if err != nil {
					readyErrors <- err
					return
				}
				if err := options.OnPeerReady(peer); err != nil {
					readyErrors <- err
					return
				}
			}
			if err := app.SendReports(options.TestReports); err != nil {
				readyErrors <- err
				return
			}
			if options.InputReports != nil {
				if err := options.InputReports(ctx, app.SendReport); err != nil && !errors.Is(err, context.Canceled) {
					readyErrors <- err
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-readyErrors:
		return err
	}
}

func ConnectedPeer(ctx context.Context, conn *dbus.Conn, adapter string) (Peer, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	call := conn.Object("org.bluez", "/").CallWithContext(ctx, ObjectManagerInterface+".GetManagedObjects", 0)
	if call.Err != nil {
		return Peer{}, call.Err
	}
	if err := call.Store(&objects); err != nil {
		return Peer{}, err
	}

	adapterPrefix := "/org/bluez/" + adapter + "/dev_"
	type peerCandidate struct {
		path dbus.ObjectPath
		peer Peer
	}
	candidates := make([]peerCandidate, 0)
	for path, interfaces := range objects {
		if !strings.HasPrefix(string(path), adapterPrefix) {
			continue
		}
		properties, ok := interfaces[DeviceInterface]
		if !ok {
			continue
		}

		connected, err := boolProperty(properties, "Connected")
		if err != nil || !connected {
			continue
		}
		address, err := stringProperty(properties, "Address")
		if err != nil {
			return Peer{}, err
		}
		name := firstStringProperty(properties, "Alias", "Name")
		if name == "" {
			name = address
		}
		candidates = append(candidates, peerCandidate{
			path: path,
			peer: Peer{
				Name:         name,
				BluetoothMAC: strings.ToUpper(address),
			},
		})
	}

	if len(candidates) == 0 {
		return Peer{}, fmt.Errorf("connected Bluetooth device was not found on %s", adapter)
	}
	sort.Slice(candidates, func(left, right int) bool {
		return candidates[left].path < candidates[right].path
	})
	if len(candidates) > 1 {
		return Peer{}, fmt.Errorf("multiple connected Bluetooth devices were found on %s", adapter)
	}

	return candidates[0].peer, nil
}

func (app *HIDApplication) addCharacteristic(path dbus.ObjectPath, uuid string, flags []string, value []byte, notify bool, writable bool, protocolMode bool) {
	app.characteristics[path] = &Characteristic{
		app:          app,
		path:         path,
		uuid:         uuid,
		servicePath:  ServicePath,
		flags:        append([]string(nil), flags...),
		value:        append([]byte(nil), value...),
		notify:       notify,
		writable:     writable,
		protocolMode: protocolMode,
	}
}

func (app *HIDApplication) addDescriptor(path dbus.ObjectPath, uuid string, characteristicPath dbus.ObjectPath, flags []string, value []byte) {
	app.descriptors[path] = &Descriptor{
		path:               path,
		uuid:               uuid,
		characteristicPath: characteristicPath,
		flags:              append([]string(nil), flags...),
		value:              append([]byte(nil), value...),
	}
}

func (app *HIDApplication) serviceProperties(interfaceName string) (map[string]dbus.Variant, bool) {
	if interfaceName != GATTServiceInterface {
		return nil, false
	}

	app.mu.Lock()
	defer app.mu.Unlock()

	return app.service.properties(), true
}

func (app *HIDApplication) notifyInputLocked(report []byte) error {
	preferredPath := ReportPath
	fallbackPath := BootInputPath
	if protocolMode := app.characteristics[ProtocolModePath]; protocolMode != nil && len(protocolMode.value) > 0 && protocolMode.value[0] == 0x00 {
		preferredPath = BootInputPath
		fallbackPath = ReportPath
	}
	if app.isNotifyingLocked(preferredPath) {
		return app.notifyLocked(preferredPath, report)
	}
	if app.isNotifyingLocked(fallbackPath) {
		return app.notifyLocked(fallbackPath, report)
	}

	return nil
}

func (app *HIDApplication) isNotifyingLocked(path dbus.ObjectPath) bool {
	characteristic := app.characteristics[path]

	return characteristic != nil && characteristic.notifying
}

func (app *HIDApplication) notifyLocked(path dbus.ObjectPath, report []byte) error {
	characteristic := app.characteristics[path]
	if characteristic == nil {
		return fmt.Errorf("missing notify characteristic: %s", path)
	}

	characteristic.value = append(characteristic.value[:0], report...)
	if app.emitter == nil {
		return nil
	}

	return app.emitter.Emit(
		path,
		PropertiesInterface+".PropertiesChanged",
		GATTCharacteristicInterface,
		map[string]dbus.Variant{"Value": dbus.MakeVariant(append([]byte(nil), report...))},
		[]string{},
	)
}

func (service *Service) properties() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"UUID":    dbus.MakeVariant(service.uuid),
		"Primary": dbus.MakeVariant(service.primary),
	}
}

func (characteristic *Characteristic) ReadValue(options map[string]dbus.Variant) ([]byte, *dbus.Error) {
	characteristic.app.mu.Lock()
	defer characteristic.app.mu.Unlock()

	value, err := readWithOffset(characteristic.value, options)
	if err != nil {
		return nil, dbusError("org.bluez.Error.InvalidOffset", err)
	}

	return value, nil
}

func (characteristic *Characteristic) WriteValue(value []byte, _ map[string]dbus.Variant) *dbus.Error {
	characteristic.app.mu.Lock()
	defer characteristic.app.mu.Unlock()

	if !characteristic.writable {
		return dbusError("org.bluez.Error.NotPermitted", errors.New("characteristic is not writable"))
	}
	if characteristic.protocolMode && len(value) != 1 {
		return dbusError("org.bluez.Error.InvalidValueLength", errors.New("protocol mode must be one byte"))
	}
	if characteristic.protocolMode && value[0] > 1 {
		return dbusError("org.bluez.Error.InvalidValueLength", errors.New("protocol mode must be 0 or 1"))
	}
	characteristic.value = append(characteristic.value[:0], value...)

	return nil
}

func (characteristic *Characteristic) StartNotify() *dbus.Error {
	characteristic.app.mu.Lock()
	defer characteristic.app.mu.Unlock()

	if !characteristic.notify {
		return dbusError("org.bluez.Error.NotSupported", errors.New("characteristic does not support notify"))
	}
	characteristic.notifying = true
	characteristic.app.subscribeOnce.Do(func() {
		close(characteristic.app.subscribed)
	})

	return nil
}

func (characteristic *Characteristic) StopNotify() *dbus.Error {
	characteristic.app.mu.Lock()
	defer characteristic.app.mu.Unlock()

	characteristic.notifying = false

	return nil
}

func (characteristic *Characteristic) Confirm() *dbus.Error {
	return nil
}

func (characteristic *Characteristic) propertiesForInterface(interfaceName string) (map[string]dbus.Variant, bool) {
	if interfaceName != GATTCharacteristicInterface {
		return nil, false
	}

	characteristic.app.mu.Lock()
	defer characteristic.app.mu.Unlock()

	return characteristic.propertiesLocked(), true
}

func (characteristic *Characteristic) propertiesLocked() map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		"UUID":    dbus.MakeVariant(characteristic.uuid),
		"Service": dbus.MakeVariant(characteristic.servicePath),
		"Flags":   dbus.MakeVariant(append([]string(nil), characteristic.flags...)),
		"Value":   dbus.MakeVariant(append([]byte(nil), characteristic.value...)),
	}
	if characteristic.notify {
		props["Notifying"] = dbus.MakeVariant(characteristic.notifying)
	}

	return props
}

func (descriptor *Descriptor) ReadValue(options map[string]dbus.Variant) ([]byte, *dbus.Error) {
	value, err := readWithOffset(descriptor.value, options)
	if err != nil {
		return nil, dbusError("org.bluez.Error.InvalidOffset", err)
	}

	return value, nil
}

func (descriptor *Descriptor) WriteValue(_ []byte, _ map[string]dbus.Variant) *dbus.Error {
	return dbusError("org.bluez.Error.NotPermitted", errors.New("descriptor is not writable"))
}

func (descriptor *Descriptor) propertiesForInterface(interfaceName string) (map[string]dbus.Variant, bool) {
	if interfaceName != GATTDescriptorInterface {
		return nil, false
	}

	return descriptor.properties(), true
}

func (descriptor *Descriptor) properties() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"UUID":           dbus.MakeVariant(descriptor.uuid),
		"Characteristic": dbus.MakeVariant(descriptor.characteristicPath),
		"Flags":          dbus.MakeVariant(append([]string(nil), descriptor.flags...)),
		"Value":          dbus.MakeVariant(append([]byte(nil), descriptor.value...)),
	}
}

func readWithOffset(value []byte, options map[string]dbus.Variant) ([]byte, error) {
	offset := uint16(0)
	if variant, ok := options["offset"]; ok {
		if err := variant.Store(&offset); err != nil {
			return nil, fmt.Errorf("read offset: %w", err)
		}
	}
	if int(offset) > len(value) {
		return nil, fmt.Errorf("offset %d is beyond value length %d", offset, len(value))
	}

	return append([]byte(nil), value[offset:]...), nil
}

func reportMap() []byte {
	return []byte{
		0x05, 0x01,
		0x09, 0x06,
		0xa1, 0x01,
		0x05, 0x07,
		0x19, 0xe0,
		0x29, 0xe7,
		0x15, 0x00,
		0x25, 0x01,
		0x75, 0x01,
		0x95, 0x08,
		0x81, 0x02,
		0x95, 0x01,
		0x75, 0x08,
		0x81, 0x01,
		0x95, 0x05,
		0x75, 0x01,
		0x05, 0x08,
		0x19, 0x01,
		0x29, 0x05,
		0x91, 0x02,
		0x95, 0x01,
		0x75, 0x03,
		0x91, 0x01,
		0x95, 0x06,
		0x75, 0x08,
		0x15, 0x00,
		0x25, 0x65,
		0x05, 0x07,
		0x19, 0x00,
		0x29, 0x65,
		0x81, 0x00,
		0xc0,
	}
}

func dbusError(name string, err error) *dbus.Error {
	return dbus.NewError(name, []any{err.Error()})
}

func boolProperty(properties map[string]dbus.Variant, name string) (bool, error) {
	var value bool
	variant, ok := properties[name]
	if !ok {
		return false, fmt.Errorf("missing Bluetooth device property: %s", name)
	}
	if err := variant.Store(&value); err != nil {
		return false, fmt.Errorf("read Bluetooth device property %s: %w", name, err)
	}

	return value, nil
}

func stringProperty(properties map[string]dbus.Variant, name string) (string, error) {
	var value string
	variant, ok := properties[name]
	if !ok {
		return "", fmt.Errorf("missing Bluetooth device property: %s", name)
	}
	if err := variant.Store(&value); err != nil {
		return "", fmt.Errorf("read Bluetooth device property %s: %w", name, err)
	}

	return value, nil
}

func firstStringProperty(properties map[string]dbus.Variant, names ...string) string {
	for _, name := range names {
		value, err := stringProperty(properties, name)
		if err == nil {
			return value
		}
	}

	return ""
}

func logf(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}

	_, _ = fmt.Fprintf(writer, format, args...)
}
