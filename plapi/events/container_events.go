package events



// Possible events received from portlater.
const (
	ContainerCreated       = "Created"
	ContainerShutdown      = "Shutdown"
	ContainerPoweredOn     = "PoweredOn"
	ContainerPoweredOff    = "PoweredOff"
	ContainerSuspended     = "Suspended"
	ContainerResumed       = "Resumed"
	ContainerRemoved       = "Removed"
	ContainerReconfigured  = "Reconfigured"
	ContainerStarted       = "Started"
	ContainerStopped       = "Stopped"
	ContainerMigrated      = "Migrated"
	ContainerMigratedByDrs = "MigratedByDrs"
	ContainerRelocated     = "Relocated"
)
