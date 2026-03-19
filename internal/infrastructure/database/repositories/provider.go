package repositories

import (
	"database/sql"

	"github.com/jsolteam/tenex-platform/internal/domain/config"
	"github.com/jsolteam/tenex-platform/internal/domain/intake"
	"github.com/jsolteam/tenex-platform/internal/domain/media"
	"github.com/jsolteam/tenex-platform/internal/domain/medicine"
	"github.com/jsolteam/tenex-platform/internal/domain/reminder"
	"github.com/jsolteam/tenex-platform/internal/domain/schedule"
	"github.com/jsolteam/tenex-platform/internal/domain/user"
	"github.com/jsolteam/tenex-platform/internal/domain/watcher"

	configrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/config"
	intakerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/intake"
	mediarepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/media"
	medicinerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/medicine"
	reminderrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/reminder"
	schedulerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/schedule"
	userrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/user"
	watcherrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/watcher"

	"github.com/jsolteam/tenex-platform/internal/platform/logger/core"
	"github.com/jsolteam/tenex-platform/internal/platform/observability/tracing"
)

// Repositories — все репозитории платформы.
type Repositories struct {
	User      user.Repository
	UserStats user.StatisticsRepository
	Media     media.Repository
	Medicine  medicine.Repository
	Schedule  schedule.Repository
	Reminder  reminder.Repository
	Intake    intake.Repository
	Watcher   watcher.Repository
	Config    config.Repository
}

func New(db *sql.DB, log *core.Logger, tracer tracing.Tracer) *Repositories {
	return &Repositories{
		User:      userrepo.New(db, log, tracer),
		UserStats: userrepo.NewStatistics(db, log, tracer),
		Media:     mediarepo.New(db, log, tracer),
		Medicine:  medicinerepo.New(db, log, tracer),
		Schedule:  schedulerepo.New(db, log, tracer),
		Reminder:  reminderrepo.New(db, log, tracer),
		Intake:    intakerepo.New(db, log, tracer),
		Watcher:   watcherrepo.New(db, log, tracer),
		Config:    configrepo.New(db, log, tracer),
	}
}
