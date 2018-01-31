package err

import "sync"

// Close close app
type Close struct {
	Mu   *sync.Mutex
	Wg   *sync.WaitGroup
	Exit *[]chan bool
}
