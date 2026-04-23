package libbox

// Keep empty on Android: go1.24+/1.25 toolchains may not expose
// os.checkPidfdOnce symbol for linkname in all host/cross combinations.
func init() {}