// SPDX-License-Identifier: GPL-3.0-only

package tui

// newScreen constructs a fresh Screen for id, wired to services (see
// screen.go's doc comment on why screens are stateless and rebuilt on
// every navigation rather than kept alive in the background).
func newScreen(id ScreenID, services *Services) Screen {
	switch id {
	case ScreenDashboard:
		return newDashboardScreen(services)
	case ScreenModelPicker:
		return newModelPickerScreen(services)
	case ScreenRoleMapper:
		return newRoleMapperScreen(services)
	case ScreenProfiles:
		return newProfilesScreen(services)
	case ScreenLiveLog:
		return newLiveLogScreen(services)
	case ScreenPlatforms:
		return newPlatformsScreen(services)
	case ScreenDoctor:
		return newDoctorScreen(services)
	default:
		return newDashboardScreen(services)
	}
}
