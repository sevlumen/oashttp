package oashttp

import internaloperation "github.com/sevlumen/oashttp/v2/internal/operation"

func (a *App) registerGroupOperation(group *Group, def *internaloperation.Definition) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.frozen {
		panic(ErrFrozen)
	}
	def.Middlewares = append([]Middleware(nil), group.middlewares...)
	a.operations = append(a.operations, def)
}
