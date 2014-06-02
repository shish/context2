package viewer

type DataSettings struct {
	// rendering
	RenderStart float64
	RenderLen   float64
	RenderScale float64

	// data loading
	MaxDepth    int
	Cutoff      float64
	Coalesce    float64
}
