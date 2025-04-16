package spider

import "sync"

type ResourceRepository interface {
	Set(req map[string]*ResourceSpec)
	Add(req *ResourceSpec)
	Delete(name string)
	HasResource(name string) bool
}

type resourceRepository struct {
	resources map[string]*ResourceSpec
	rlock     sync.Mutex
}

/*
无输入，输出一个ResourceRepository实例

该方法用于创建一个ResourceRepository实例，返回实例的指针
*/
func NewResourceRepository() ResourceRepository {
	r := &resourceRepository{}
	r.resources = make(map[string]*ResourceSpec, 100)
	return r
}

/*
输入一个资源列表，无输出

该方法用于设置资源列表，将传入的资源列表设置为当前资源列表，全量替换当前所有资源
*/
func (r *resourceRepository) Set(req map[string]*ResourceSpec) {
	r.rlock.Lock()
	defer r.rlock.Unlock()
	r.resources = req
}

/*
输入一个资源规格，无输出

该方法用于添加一个资源规格，将传入的资源规格添加到当前资源列表中
*/
func (r *resourceRepository) Add(req *ResourceSpec) {
	r.rlock.Lock()
	defer r.rlock.Unlock()
	r.resources[req.Name] = req
}

/*
输入一个资源名称，无输出

该方法用于删除一个资源，根据传入的资源名称，从当前资源列表中删除对应的资源
*/
func (r *resourceRepository) Delete(name string) {
	r.rlock.Lock()
	defer r.rlock.Unlock()
	delete(r.resources, name)
}

/*
输入一个资源名称，输出一个布尔值

该方法用于判断当前资源列表中是否存在指定的资源，根据传入的资源名称，判断当前资源列表中是否存在对应的资源，返回布尔值表示结果
*/
func (r *resourceRepository) HasResource(name string) bool {
	r.rlock.Lock()
	defer r.rlock.Unlock()
	if _, ok := r.resources[name]; ok {
		return true
	}

	return false
}
