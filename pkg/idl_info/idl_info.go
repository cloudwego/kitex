package idl_info

type IDLInfo struct {
	IDLInfoMeta
	IDLFileContentMap map[string]string `json:"idl_file_content_map"`
}

type IDLInfoMeta struct {
	PSM         string `json:"psm"`
	MainIDLPath string `json:"mainIDLPath"`
	GoModule    string `json:"go_module"`
	CommitID    string `json:"commit_id"`
}

func GetIDLInfoDetails(psm string) (IDLInfo, bool) {
	// todo: 拿副本？保证 map 不能被修改？
	groups, ok := globalManager[psm]
	if !ok {
		return IDLInfo{}, false
	}
	return *groups, true
}

func GetIDLInfoList() []IDLInfoMeta {
	var metas = make([]IDLInfoMeta, 0, len(globalManager))
	for _, info := range globalManager {
		metas = append(metas, info.IDLInfoMeta)
	}
	return metas
}

var (
	globalManager = make(map[string]*IDLInfo)
)

func RegisterIDL(psm, goModule string, isMainIDL bool, idlContent, idlPath string) {
	// todo 后面区分下 thrift 和 protobuf
	// 注册对启动速度的影响，init 不会并发
	if _, ok := globalManager[psm]; !ok {
		globalManager[psm] = &IDLInfo{
			IDLInfoMeta: IDLInfoMeta{
				PSM: psm,
			},
			IDLFileContentMap: make(map[string]string),
		}
	}
	group := globalManager[psm]
	group.IDLFileContentMap[idlPath] = idlContent
	if group.GoModule != "" && group.GoModule != goModule {
		// 如果多个项目重复注册了一样的 psm，应该及时报错
		panic("duplicated register for psm xx")
	} else {
		group.GoModule = goModule
	}
	if isMainIDL {
		group.MainIDLPath = idlPath
	}
	// todo 注册的时候各种冲突场景路径信息打印等等...更多模块名元信息记录gopath啥的？
	// todo 看看 protobuf 的全局注册器的实现
}
