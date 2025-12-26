package importer

type NoOpProgressReporter struct{}

func (r *NoOpProgressReporter) Start(total int, message string)    {}
func (r *NoOpProgressReporter) Update(current int, message string) {}
func (r *NoOpProgressReporter) Finish(message string)              {}
func (r *NoOpProgressReporter) Error(err error)                    {}
