package err

import "sync"

// Close close app
type Close struct {
	Wg   *sync.WaitGroup
	Exit *[]chan bool
}
