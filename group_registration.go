package oashttp

import internaloperation "github.com/sevlumen/oashttp/v2/internal/operation"

func (a *App) registerGroupOperation(group *Group, def *internaloperation.Definition) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		panic(ErrFrozen)
	}
	def.Middlewares = nil
	for _, middleware := range group.middlewares {
		def.Middlewares = append(def.Middlewares, middleware)
	}
	a.operations = append(a.operations, def)
}
