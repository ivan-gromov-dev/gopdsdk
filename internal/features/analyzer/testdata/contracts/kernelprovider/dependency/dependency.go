package dependency

// Never returns only by panicking, allowing ctrlflow to export a no-return fact.
func Never() { panic("stop") }
