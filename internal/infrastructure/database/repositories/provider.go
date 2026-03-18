package repositories

import (
	"database/sql"

	configrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/config"
	intakerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/intake"
	mediarepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/media"
	medicinerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/medicine"
	reminderrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/reminder"
	schedulerepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/schedule"
	userrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/user"
	watcherrepo "github.com/jsolteam/tenex-platform/internal/infrastructure/database/repositories/watcher"
)

// Repositories — все репозитории платформы.
type Repositories struct {
	User      *userrepo.Repository
	UserStats *userrepo.StatisticsRepository
	Media     *mediarepo.Repository
	Medicine  *medicinerepo.Repository
	Schedule  *schedulerepo.Repository
	Reminder  *reminderrepo.Repository
	Intake    *intakerepo.Repository
	Watcher   *watcherrepo.Repository
	Config    *configrepo.Repository
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		User:      userrepo.New(db),
		UserStats: userrepo.NewStatistics(db),
		Media:     mediarepo.New(db),
		Medicine:  medicinerepo.New(db),
		Schedule:  schedulerepo.New(db),
		Reminder:  reminderrepo.New(db),
		Intake:    intakerepo.New(db),
		Watcher:   watcherrepo.New(db),
		Config:    configrepo.New(db),
	}
}
