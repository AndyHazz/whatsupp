package hub

import (
	"github.com/andyhazz/whatsupp/internal/store"
)

type IncidentManager struct {
	store *store.Store
}

func NewIncidentManager(s *store.Store) *IncidentManager {
	return &IncidentManager{store: s}
}

func (im *IncidentManager) HandleTransition(monitor string, transition Transition, timestamp int64, cause string) (*store.Incident, error) {
	switch transition {
	case TransitionToDown:
		id, err := im.store.CreateIncident(monitor, timestamp, cause)
		if err != nil {
			return nil, err
		}
		return &store.Incident{
			ID:        id,
			Monitor:   monitor,
			StartedAt: timestamp,
			Cause:     cause,
		}, nil

	case TransitionToUp:
		inc, err := im.store.GetOpenIncident(monitor)
		if err != nil {
			return nil, err
		}
		if inc == nil {
			return nil, nil
		}
		if err := im.store.ResolveIncident(inc.ID, timestamp); err != nil {
			return nil, err
		}
		inc.ResolvedAt = &timestamp
		return inc, nil

	default:
		return nil, nil
	}
}
