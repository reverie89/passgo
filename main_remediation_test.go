package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSortVMsNumericColumns(t *testing.T) {
	t.Run("snapshots ascending numeric", func(t *testing.T) {
		vms := []vmData{
			{info: VMInfo{Name: "vm-10", Snapshots: "10"}},
			{info: VMInfo{Name: "vm-2", Snapshots: "2"}},
			{info: VMInfo{Name: "vm-1", Snapshots: "1"}},
		}

		sortVMs(vms, 2, true)

		got := []string{vms[0].info.Name, vms[1].info.Name, vms[2].info.Name}
		want := []string{"vm-1", "vm-2", "vm-10"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unexpected order: got %v want %v", got, want)
		}
	})

	t.Run("snapshots descending numeric", func(t *testing.T) {
		vms := []vmData{
			{info: VMInfo{Name: "vm-2", Snapshots: "2"}},
			{info: VMInfo{Name: "vm-1", Snapshots: "1"}},
			{info: VMInfo{Name: "vm-10", Snapshots: "10"}},
		}

		sortVMs(vms, 2, false)

		got := []string{vms[0].info.Name, vms[1].info.Name, vms[2].info.Name}
		want := []string{"vm-10", "vm-2", "vm-1"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unexpected order: got %v want %v", got, want)
		}
	})

	t.Run("deterministic tie-break by name", func(t *testing.T) {
		vms := []vmData{
			{info: VMInfo{Name: "vm-b", CPUs: "2"}},
			{info: VMInfo{Name: "vm-a", CPUs: "2"}},
			{info: VMInfo{Name: "vm-c", CPUs: "2"}},
		}

		sortVMs(vms, 4, true)

		got := []string{vms[0].info.Name, vms[1].info.Name, vms[2].info.Name}
		want := []string{"vm-a", "vm-b", "vm-c"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unexpected tie-break order: got %v want %v", got, want)
		}
	})
}

func TestRootModelAutoRefreshCoalescesInFlightFetches(t *testing.T) {
	m := rootModel{
		currentView:             viewTable,
		table:                   newTableModel(),
		vmListFetchInFlight:     true,
		vmListFetchPending:      false,
		vmListPendingBackground: false,
	}

	model, _ := m.Update(autoRefreshTickMsg(time.Now()))
	m1 := model.(rootModel)

	if !m1.vmListFetchInFlight {
		t.Fatalf("expected in-flight fetch to remain true")
	}
	if !m1.vmListFetchPending {
		t.Fatalf("expected pending fetch to be queued")
	}
	if !m1.vmListPendingBackground {
		t.Fatalf("expected pending fetch to remain background")
	}

	model, _ = m1.Update(vmListResultMsg{
		vms:        []vmData{{info: VMInfo{Name: "vm-1"}}},
		background: true,
	})
	m2 := model.(rootModel)

	if !m2.vmListFetchInFlight {
		t.Fatalf("expected queued refresh to be scheduled after completion")
	}
	if m2.vmListFetchPending {
		t.Fatalf("expected pending flag to be cleared once scheduled")
	}
}

func TestRootModelDropsPendingBackgroundRefreshOffTable(t *testing.T) {
	m := rootModel{
		currentView:         viewTable,
		table:               newTableModel(),
		vmListFetchInFlight: true,
	}

	model, _ := m.Update(autoRefreshTickMsg(time.Now()))
	m1 := model.(rootModel)
	m1.currentView = viewInfo

	model, _ = m1.Update(vmListResultMsg{background: true})
	m2 := model.(rootModel)

	if m2.vmListFetchInFlight {
		t.Fatalf("expected no background refresh while off table view")
	}
	if m2.vmListFetchPending {
		t.Fatalf("expected pending flag to be cleared")
	}
}

func TestRunMountModifyOperation(t *testing.T) {
	t.Run("unmount failure short-circuits remount", func(t *testing.T) {
		var calls [][]string
		runCmd := func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			if args[0] == "umount" {
				return "", errors.New("unmount failed")
			}
			return "", nil
		}

		err := runMountModifyOperation(runCmd, "vm1", "/old", "/new-src", "/new")
		if err == nil || !strings.Contains(err.Error(), "failed to unmount") {
			t.Fatalf("expected unmount failure, got: %v", err)
		}
		if len(calls) != 1 {
			t.Fatalf("expected only umount call, got %d calls", len(calls))
		}
	})

	t.Run("remount failure is surfaced", func(t *testing.T) {
		var calls [][]string
		runCmd := func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			if args[0] == "mount" {
				return "", errors.New("mount failed")
			}
			return "", nil
		}

		err := runMountModifyOperation(runCmd, "vm1", "/old", "/new-src", "/new")
		if err == nil || !strings.Contains(err.Error(), "failed to mount") {
			t.Fatalf("expected mount failure, got: %v", err)
		}
		if len(calls) != 2 {
			t.Fatalf("expected umount+mount calls, got %d calls", len(calls))
		}
	})
}
