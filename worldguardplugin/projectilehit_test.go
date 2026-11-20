package worldguardplugin

import (
	"testing"

	"github.com/golem-mc/golem/plugin"
)

// unboundProjectile panics on every method, like golem's placeholder for a
// projectile whose body is gone.
type unboundProjectile struct{ plugin.Projectile }

func TestUnboundProjectileHitDoesNotPanic(t *testing.T) {
	if isBoundBobber(unboundProjectile{}) {
		t.Fatal("unbound projectile taken for a bobber")
	}
	if isBoundBobber(nil) {
		t.Fatal("nil projectile taken for a bobber")
	}
}
