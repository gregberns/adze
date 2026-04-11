package cli

// Exit codes per the CLI Surface spec.
const (
	ExitSuccess        = 0
	ExitUnexpected     = 1
	ExitConfigError    = 2
	ExitPreFlightFail  = 3
	ExitExecFailure    = 4
	ExitPartialSuccess = 5
	ExitChangesPlanned = 6
	ExitDriftDetected  = 7
)
