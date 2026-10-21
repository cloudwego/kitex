package remote

import (
	"context"
	"unsafe"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
)

type HeaderEntry = struct {
	Kind, ID   uint16
	Op         uint32
	Key, Value string
}

type HeaderView struct {
	entries       []HeaderEntry
	metadataIndex [21]uint32
	byteSize      int
	valid         bool
}

func (s *HeaderView) Valid() bool { return s != nil && s.valid }
func (s *HeaderView) Len() int {
	if s == nil {
		return 0
	}
	return len(s.entries)
}
func HeaderViewLayout() [6]uintptr {
	return [6]uintptr{unsafe.Sizeof(HeaderView{}), unsafe.Alignof(HeaderView{}), unsafe.Offsetof(HeaderView{}.entries), unsafe.Offsetof(HeaderView{}.metadataIndex), unsafe.Offsetof(HeaderView{}.byteSize), unsafe.Offsetof(HeaderView{}.valid)}
}

type headerStore struct {
	view               HeaderView
	intFast            [33]uint32
	strFast            [7]uint32
	intSlots           map[uint16]int
	strSlots           map[string]int
	intCount, strCount int
	gdprIndex          uint32
	response           bool
	trackNativeChanges bool
	changedIndices     []int
	changeView         HeaderView
}

var headerMetadataSlot = [29]uint8{2: 12, 3: 1, 4: 3, 5: 5, 6: 2, 7: 4, 8: 6, 9: 9, 10: 10, 11: 13, 14: 14, 16: 19, 19: 20, 20: 8, 21: 18, 24: 15, 25: 21, 26: 11, 28: 7}

func (s *headerStore) reset(response bool) {
	s.clearNativeChanges()
	s.trackNativeChanges = false
	for i := range s.view.entries {
		s.view.entries[i] = HeaderEntry{}
	}
	for key := range s.intSlots {
		delete(s.intSlots, key)
	}
	for key := range s.strSlots {
		delete(s.strSlots, key)
	}
	s.intFast, s.strFast = [33]uint32{}, [7]uint32{}
	s.view = HeaderView{entries: s.view.entries[:0], valid: true}
	s.intCount, s.strCount, s.gdprIndex = 0, 0, 0
	s.response = response
}

func entryBytes(e HeaderEntry) int { return len(e.Key) + len(e.Value) + 2 }

func (s *headerStore) kinds() (uint16, uint16) {
	if s.response {
		return 4, 10
	}
	return 2, 3
}

func headerStringSlot(key string) uint8 {
	switch key {
	case "mct":
		return 1
	case "mtt":
		return 2
	case "rip":
		return 3
	case "isn":
		return 4
	case "RPC_TRANSIT_gdpr-token":
		return 5
	case "RPC_TRANSIT_secondary-acl-token":
		return 6
	case "RPC_TRANSIT_EGRESS_SENT_TS":
		return 7
	}
	return 0
}

func (s *headerStore) intSlot(id uint16) (int, bool) {
	if id < uint16(len(s.intFast)) {
		index := s.intFast[id]
		return int(index) - 1, index != 0
	}
	index, found := s.intSlots[id]
	return index, found
}

func (s *headerStore) stringSlot(key string) (int, bool) {
	if slot := headerStringSlot(key); slot != 0 {
		index := s.strFast[slot-1]
		return int(index) - 1, index != 0
	}
	index, found := s.strSlots[key]
	return index, found
}

func (s *headerStore) lookup(integer bool, id uint16, key string) (int, bool) {
	if integer {
		return s.intSlot(id)
	}
	return s.stringSlot(key)
}

func (s *headerStore) set(integer bool, id uint16, key, value string, deleted bool) bool {
	index, found := s.lookup(integer, id, key)
	active := found && s.view.entries[index].Op == 0
	if deleted && !active {
		return true
	}
	if !deleted && active && s.view.entries[index].Value == value {
		return true
	}
	if len(key) > 16<<20 || len(value) > 16<<20 || (!found && len(s.view.entries) == 65535) {
		return false
	}
	stringKind, intKind := s.kinds()
	e := HeaderEntry{Kind: stringKind, ID: id, Key: key, Value: value}
	if integer {
		e.Kind = intKind
	}
	if deleted {
		e.Op, e.Value = 1, ""
	}
	size := s.view.byteSize + entryBytes(e)
	if found {
		size -= entryBytes(s.view.entries[index])
	}
	if size > 64<<20 {
		return false
	}
	if found {
		s.view.entries[index] = e
	} else {
		index = len(s.view.entries)
		s.view.entries = append(s.view.entries, e)
		if integer {
			if id < uint16(len(s.intFast)) {
				s.intFast[id] = uint32(index + 1)
			} else {
				if s.intSlots == nil {
					s.intSlots = make(map[uint16]int)
				}
				s.intSlots[id] = index
			}
		} else if slot := headerStringSlot(key); slot != 0 {
			s.strFast[slot-1] = uint32(index + 1)
		} else {
			if s.strSlots == nil {
				s.strSlots = make(map[string]int)
			}
			s.strSlots[key] = index
		}
	}
	if s.trackNativeChanges {
		s.recordNativeChange(index)
	}
	s.view.byteSize = size
	change := 0
	if !active && !deleted {
		change = 1
	}
	if active && deleted {
		change = -1
	}
	if integer {
		s.intCount += change
	} else {
		s.strCount += change
	}
	if !integer && key == ttheader.GDPRToken {
		s.gdprIndex = uint32(index + 1)
		if deleted {
			s.gdprIndex = 0
		}
	}
	if !s.response {
		slot := uint8(0)
		if integer && int(id) < len(headerMetadataSlot) {
			slot = headerMetadataSlot[id]
		}
		if !integer && key == ttheader.GDPRToken {
			slot = 16
		}
		if !integer && key == "RPC_TRANSIT_secondary-acl-token" {
			slot = 17
		}
		if slot != 0 {
			s.view.metadataIndex[slot-1] = uint32(index + 1)
			if deleted {
				s.view.metadataIndex[slot-1] = 0
			}
		}
	}
	return true
}

func (ti *transInfo) materialize() {
	if ti.intInfo == nil {
		ti.intInfo = make(map[uint16]string)
	}
	if ti.strInfo == nil {
		ti.strInfo = make(map[string]string)
	}
	if ti.primary {
		stringKind, _ := ti.store.kinds()
		for _, e := range ti.store.view.entries {
			if e.Op != 0 {
				continue
			}
			if e.Kind == stringKind {
				ti.strInfo[e.Key] = e.Value
			} else {
				ti.intInfo[e.ID] = e.Value
			}
		}
		ti.store.reset(ti.store.response)
	}
	ti.primary, ti.escaped, ti.fresh = false, true, false
}

type HeaderAccess struct{ info TransInfo }

func AccessHeaders(info TransInfo) HeaderAccess { return HeaderAccess{info: info} }

func (a HeaderAccess) EnablePrimary(response bool) bool {
	ti, ok := a.info.(*transInfo)
	if !ok {
		return false
	}
	if ti.primary {
		if ti.store.response == response {
			return true
		}
		ti.materialize()
		return false
	}
	if ti.escaped && ti.fresh && len(ti.intInfo) == 0 && len(ti.strInfo) == 0 {
		ti.intInfo, ti.strInfo, ti.escaped = nil, nil, false
	}
	if ti.escaped || len(ti.intInfo) != 0 || len(ti.strInfo) != 0 {
		return false
	}
	if ti.store == nil {
		ti.store = new(headerStore)
	}
	ti.store.reset(response)
	ti.primary = true
	return true
}

func (a HeaderAccess) Primary() bool {
	ti, ok := a.info.(*transInfo)
	return ok && ti.primary
}

func (a HeaderAccess) View() *HeaderView {
	if ti, ok := a.info.(*transInfo); ok && ti.primary {
		return &ti.store.view
	}
	return nil
}

func (a HeaderAccess) Int(id uint16) string     { value, _ := a.IntValue(id); return value }
func (a HeaderAccess) String(key string) string { value, _ := a.StringValue(key); return value }

func (a HeaderAccess) IntValue(id uint16) (string, bool) {
	if a.info == nil {
		return "", false
	}
	if ti, ok := a.info.(*transInfo); ok {
		if ti.primary {
			index, found := ti.store.intSlot(id)
			if !found || ti.store.view.entries[index].Op != 0 {
				return "", false
			}
			return ti.store.view.entries[index].Value, true
		}
		value, found := ti.intInfo[id]
		return value, found
	}
	value, found := a.info.TransIntInfo()[id]
	return value, found
}

func (a HeaderAccess) StringValue(key string) (string, bool) {
	if a.info == nil {
		return "", false
	}
	if ti, ok := a.info.(*transInfo); ok {
		if ti.primary {
			index, found := ti.store.stringSlot(key)
			if !found || ti.store.view.entries[index].Op != 0 {
				return "", false
			}
			return ti.store.view.entries[index].Value, true
		}
		value, found := ti.strInfo[key]
		return value, found
	}
	value, found := a.info.TransStrInfo()[key]
	return value, found
}

func (a HeaderAccess) SetInt(id uint16, value string) { a.change(true, id, "", value, false) }
func (a HeaderAccess) SetString(key, value string)    { a.change(false, 0, key, value, false) }
func (a HeaderAccess) DeleteInt(id uint16)            { a.change(true, id, "", "", true) }
func (a HeaderAccess) DeleteString(key string)        { a.change(false, 0, key, "", true) }

func (ti *transInfo) SetInt(id uint16, value string) {
	ti.fresh = false
	if ti.primary {
		if ti.store.set(true, id, "", value, false) {
			if id == 32 {
				index, found := ti.store.stringSlot(ttheader.GDPRToken)
				if found && ti.store.view.entries[index].Op == 0 {
					ti.materialize()
				}
			}
			return
		}
		ti.materialize()
	}
	if ti.intInfo == nil {
		ti.intInfo = make(map[uint16]string)
	}
	ti.intInfo[id] = value
}

func (ti *transInfo) SetString(key, value string) {
	ti.fresh = false
	if ti.primary {
		if ti.store.set(false, 0, key, value, false) {
			if key == ttheader.GDPRToken {
				index, found := ti.store.intSlot(32)
				if found && ti.store.view.entries[index].Op == 0 {
					ti.materialize()
				}
			}
			return
		}
		ti.materialize()
	}
	if ti.strInfo == nil {
		ti.strInfo = make(map[string]string)
	}
	ti.strInfo[key] = value
}

func (a HeaderAccess) change(integer bool, id uint16, key, value string, deleted bool) {
	if a.info == nil {
		return
	}
	ti, ok := a.info.(*transInfo)
	if !ok {
		if integer {
			values := a.info.TransIntInfo()
			if deleted {
				delete(values, id)
			} else if values != nil {
				values[id] = value
			} else {
				a.info.PutTransIntInfo(map[uint16]string{id: value})
			}
		} else {
			values := a.info.TransStrInfo()
			if deleted {
				delete(values, key)
			} else if values != nil {
				values[key] = value
			} else {
				a.info.PutTransStrInfo(map[string]string{key: value})
			}
		}
		return
	}
	ti.fresh = false
	if ti.primary {
		if ti.store.set(integer, id, key, value, deleted) {
			if (integer && id == 32) || (!integer && key == ttheader.GDPRToken) {
				_, intACL := a.IntValue(32)
				_, strACL := a.StringValue(ttheader.GDPRToken)
				if intACL && strACL {
					ti.materialize()
				}
			}
			return
		}
		ti.materialize()
	}
	if integer {
		if deleted {
			delete(ti.intInfo, id)
		} else {
			if ti.intInfo == nil {
				ti.intInfo = make(map[uint16]string)
			}
			ti.intInfo[id] = value
		}
	} else {
		if deleted {
			delete(ti.strInfo, key)
		} else {
			if ti.strInfo == nil {
				ti.strInfo = make(map[string]string)
			}
			ti.strInfo[key] = value
		}
	}
}

func (a HeaderAccess) MergeInt(values map[uint16]string) {
	for key, value := range values {
		a.SetInt(key, value)
	}
}

func (a HeaderAccess) MergeString(values map[string]string) {
	for key, value := range values {
		a.SetString(key, value)
	}
}

func (a HeaderAccess) RangeInt(visit func(uint16, string) bool) {
	if a.info == nil {
		return
	}
	if ti, ok := a.info.(*transInfo); ok {
		if ti.primary {
			_, kind := ti.store.kinds()
			for _, e := range ti.store.view.entries {
				if e.Op == 0 && e.Kind == kind && !visit(e.ID, e.Value) {
					return
				}
			}
			return
		}
		for key, value := range ti.intInfo {
			if !visit(key, value) {
				return
			}
		}
		return
	}
	for key, value := range a.info.TransIntInfo() {
		if !visit(key, value) {
			return
		}
	}
}

func (a HeaderAccess) RangeString(visit func(string, string) bool) {
	if a.info == nil {
		return
	}
	if ti, ok := a.info.(*transInfo); ok {
		if ti.primary {
			kind, _ := ti.store.kinds()
			for _, e := range ti.store.view.entries {
				if e.Op == 0 && e.Kind == kind && !visit(e.Key, e.Value) {
					return
				}
			}
			return
		}
		for key, value := range ti.strInfo {
			if !visit(key, value) {
				return
			}
		}
		return
	}
	for key, value := range a.info.TransStrInfo() {
		if !visit(key, value) {
			return
		}
	}
}

func (a HeaderAccess) LegacyMaps() (map[uint16]string, map[string]string) {
	if a.info == nil {
		return nil, nil
	}
	return a.info.TransIntInfo(), a.info.TransStrInfo()
}

func (a HeaderAccess) EncodeTTHeader(ctx context.Context, p ttheader.EncodeParam, out bufiox.Writer) ([]byte, error) {
	if ti, ok := a.info.(*transInfo); ok {
		if ti.primary {
			s := ti.store
			stringKind, intKind := s.kinds()
			return ttheader.EncodeEntries(ctx, p, ttheader.HeaderEntries{Entries: s.view.entries, IntCount: s.intCount, StrCount: s.strCount, GDPRIndex: s.gdprIndex, StringKind: stringKind, IntKind: intKind}, out)
		}
		p.IntInfo, p.StrInfo = ti.intInfo, ti.strInfo
	} else {
		p.IntInfo, p.StrInfo = a.LegacyMaps()
	}
	return ttheader.Encode(ctx, p, out)
}

func (a HeaderAccess) DecodeTTHeader(ctx context.Context, in bufiox.Reader) (ttheader.DecodeParam, error) {
	var p ttheader.DecodeParam
	var err error
	if ti, ok := a.info.(*transInfo); ok && ti.primary && ti.store.intCount == 0 && ti.store.strCount == 0 {
		ti.store.reset(ti.store.response)
		if _, native := in.(*nativeByteBuffer); native {
			p, err = ttheader.DecodeToOwned(ctx, in, ti)
		} else {
			p, err = ttheader.DecodeTo(ctx, in, ti)
		}
		if err != nil {
			if ti.primary {
				ti.store.reset(ti.store.response)
				ti.materialize()
			} else {
				for key := range ti.intInfo {
					delete(ti.intInfo, key)
				}
				for key := range ti.strInfo {
					delete(ti.strInfo, key)
				}
			}
		}
		return p, err
	}
	intInfo, strInfo := a.LegacyMaps()
	reuse := len(intInfo) == 0 && len(strInfo) == 0
	if reuse {
		p, err = ttheader.DecodeInto(ctx, in, intInfo, strInfo)
	} else {
		p, err = ttheader.Decode(ctx, in)
	}
	if err != nil {
		if reuse {
			for key := range intInfo {
				delete(intInfo, key)
			}
			for key := range strInfo {
				delete(strInfo, key)
			}
		}
		return p, err
	}
	if a.info != nil {
		if !reuse || intInfo == nil {
			a.info.PutTransIntInfo(p.IntInfo)
		}
		if !reuse || strInfo == nil {
			a.info.PutTransStrInfo(p.StrInfo)
		}
	}
	return p, nil
}

func (s *headerStore) recordNativeChange(index int) {
	for _, changed := range s.changedIndices {
		if changed == index {
			return
		}
	}
	s.changedIndices = append(s.changedIndices, index)
}

func (s *headerStore) clearNativeChanges() {
	for i := range s.changeView.entries {
		s.changeView.entries[i] = HeaderEntry{}
	}
	s.changeView = HeaderView{entries: s.changeView.entries[:0]}
	s.changedIndices = s.changedIndices[:0]
}

func (a HeaderAccess) ResetNativeChanges(track bool) bool {
	ti, ok := a.info.(*transInfo)
	if !ok || ti.store == nil {
		return false
	}
	s := ti.store
	s.clearNativeChanges()
	s.trackNativeChanges = track && ti.primary
	return s.trackNativeChanges
}

func (a HeaderAccess) NativeChanges() *HeaderView {
	ti, ok := a.info.(*transInfo)
	if !ok || !ti.primary || !ti.store.trackNativeChanges {
		return nil
	}
	s := ti.store
	for i := range s.changeView.entries {
		s.changeView.entries[i] = HeaderEntry{}
	}
	s.changeView.entries = s.changeView.entries[:0]
	s.changeView.byteSize = 0
	for _, index := range s.changedIndices {
		entry := s.view.entries[index]
		s.changeView.entries = append(s.changeView.entries, entry)
		s.changeView.byteSize += entryBytes(entry)
	}
	s.changeView.valid = true
	return &s.changeView
}
