package agent

// Profile defines resource limits for an agent type
type Profile struct {
	CPUs   int
	Memory string
}

// Default profiles by agent type
var profiles = map[string]Profile{
	"Explore": {
		CPUs:   2,
		Memory: "1g",
	},
	"Plan": {
		CPUs:   2,
		Memory: "1g",
	},
	"Bash": {
		CPUs:   4,
		Memory: "2g",
	},
	"code-reviewer": {
		CPUs:   2,
		Memory: "1g",
	},
	"general-purpose": {
		CPUs:   2,
		Memory: "1g",
	},
}

// DefaultProfile is used when no specific profile matches
var DefaultProfile = Profile{
	CPUs:   2,
	Memory: "1g",
}

// GetProfile returns the resource profile for an agent type
func GetProfile(agentType string) Profile {
	if profile, ok := profiles[agentType]; ok {
		return profile
	}
	return DefaultProfile
}

// SetProfile allows customizing a profile
func SetProfile(agentType string, profile Profile) {
	profiles[agentType] = profile
}
