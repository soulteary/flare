package cache

var GlobalKV *Cache

func init() {
	GlobalKV = New(NoExpiration, NoExpiration)
}

func GetBool(key string) bool {
	if v, found := GlobalKV.Get(key); found {
		return v.(bool)
	}
	return false
}
