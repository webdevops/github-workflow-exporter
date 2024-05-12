package main

import (
	"github.com/KimMachineGun/automemlimit/memlimit"
)

func initSystem() {
	// set memory limit
	memlimit.SetGoMemLimitWithOpts(
		memlimit.WithProvider(
			memlimit.ApplyFallback(
				memlimit.FromCgroup,
				memlimit.FromSystem,
			),
		),
		memlimit.WithLogger(slogger),
	)
}
