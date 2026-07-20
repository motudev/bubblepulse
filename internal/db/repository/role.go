package repository

// User roles as stored in users.role and enforced by ck_users_role.
const (
	// RoleAdmin manages the whole organization: teams, roles, and the org itself.
	RoleAdmin = "ADMIN"
	// RoleTeamEditor manages the membership of their own team.
	RoleTeamEditor = "TEAM_EDITOR"
	// RoleUpdater is the default role: posts updates and reads the dashboard.
	RoleUpdater = "UPDATER"
)

// IsValidRole reports whether role is one of the known user roles.
func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleTeamEditor || role == RoleUpdater
}
