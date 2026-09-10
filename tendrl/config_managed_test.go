package tendrl

import "testing"

// Managed mode has to mean what the documentation says it means.
//
// The regression: ConfigFile's switches were plain bools, so a host with no
// config file produced a zero-valued ConfigFile that read as an explicit
// "false" for every one and overwrote the defaults. toConfig then ran its
// headless teardown against the FILE's managed flag, before the constructor
// overrode Managed from its own argument. NewClient(true) therefore yielded a
// client reporting managed=true with offline storage, offline retry,
// connectivity checking and heartbeats all switched off.

func TestNoConfigFileLeavesManagedFeaturesOn(t *testing.T) {
	// An empty ConfigFile is exactly what a host with no config file yields.
	cfg := (&ConfigFile{}).toConfig()
	cfg.Managed = true
	cfg.ApplyManagedGating()

	for _, tc := range []struct {
		name string
		got  bool
	}{
		{"offline storage", cfg.OfflineStorage},
		{"offline retry", cfg.OfflineRetryEnabled},
		{"connectivity checking", cfg.ConnectivityCheckEnabled},
		{"heartbeats", cfg.SendHeartbeat},
	} {
		if !tc.got {
			t.Errorf("managed mode with no config file left %s off; "+
				"the docs say these are automatic in managed mode", tc.name)
		}
	}
}

func TestHeadlessStillDisablesManagedFeatures(t *testing.T) {
	cfg := (&ConfigFile{}).toConfig()
	cfg.Managed = false
	cfg.ApplyManagedGating()

	for _, tc := range []struct {
		name string
		got  bool
	}{
		{"offline storage", cfg.OfflineStorage},
		{"offline retry", cfg.OfflineRetryEnabled},
		{"connectivity checking", cfg.ConnectivityCheckEnabled},
		{"heartbeats", cfg.SendHeartbeat},
	} {
		if tc.got {
			t.Errorf("headless mode left %s on; managed-only work must not run", tc.name)
		}
	}
}

func TestAnExplicitFalseIsStillHonored(t *testing.T) {
	// The whole point of the pointers: "absent" and "explicitly off" must not
	// look the same. An operator who turns something off keeps it off.
	off := false
	cfg := (&ConfigFile{
		OfflineStorage: &off,
		SendHeartbeat:  &off,
	}).toConfig()
	cfg.Managed = true
	cfg.ApplyManagedGating()

	if cfg.OfflineStorage {
		t.Error(`"offline_storage": false was ignored`)
	}
	if cfg.SendHeartbeat {
		t.Error(`"send_heartbeat": false was ignored`)
	}
	// Untouched keys keep their managed defaults.
	if !cfg.ConnectivityCheckEnabled {
		t.Error("a key the file never mentioned was switched off anyway")
	}
}

func TestTheFilesManagedKeyStillWorksOnItsOwn(t *testing.T) {
	// NewClientWithConfigAndAPIKey has no managed argument, so the file's key
	// is the only source of truth on that path.
	off := false
	cfg := (&ConfigFile{Managed: &off}).toConfig()
	cfg.ApplyManagedGating()

	if cfg.Managed {
		t.Fatal(`"managed": false in the config file was ignored`)
	}
	if cfg.OfflineStorage || cfg.SendHeartbeat {
		t.Error("a file-declared headless client still had managed features on")
	}
}

func TestGeneratedExampleConfigRoundTrips(t *testing.T) {
	// The example is what an operator copies. It must parse back to the same
	// settings it advertises.
	cfg := GenerateExampleConfig().toConfig()
	cfg.ApplyManagedGating()

	if !cfg.Managed || !cfg.OfflineStorage || !cfg.OfflineRetryEnabled ||
		!cfg.ConnectivityCheckEnabled || !cfg.SendHeartbeat {
		t.Error("the generated example config does not produce the settings it lists")
	}
}
