package health

import "context"

type Checker interface {
	Name() string
	Check(context.Context) error
}

type Dependency struct {
	Status string `json:"status"`
}

type Report struct {
	Ready        bool                  `json:"-"`
	Dependencies map[string]Dependency `json:"dependencies"`
}

type Service struct {
	checkers []Checker
}

func NewService(checkers []Checker) *Service {
	return &Service{checkers: checkers}
}

func (service *Service) Check(context context.Context) Report {
	report := Report{
		Ready:        true,
		Dependencies: make(map[string]Dependency, len(service.checkers)),
	}
	for _, checker := range service.checkers {
		if err := checker.Check(context); err != nil {
			report.Ready = false
			report.Dependencies[checker.Name()] = Dependency{Status: "unavailable"}
			continue
		}
		report.Dependencies[checker.Name()] = Dependency{Status: "ok"}
	}
	return report
}
