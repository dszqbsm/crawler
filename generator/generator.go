package generator

import (
	"bytes"
	"encoding/binary"
	"net"
)

/*
输入一个IP地址，输出一个32位整数

解析IP地址为net.IP类型，提取IP的4字节，将字节流转换为unit32类型
*/
func IDbyIP(ip string) uint32 {
	var id uint32
	// 将IPv4地址转换为32位整数
	binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &id)
	return id
}
