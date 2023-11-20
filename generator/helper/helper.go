/**
 * @author   Liang
 * @create   2021/7/4 0004 10:09
 * @version  1.0
 */
package helper

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/tool/protobuf/pkg/typemap"

	"github.com/imroc/biu"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type Charset string

const (
	UTF8    = Charset("UTF-8")
	GB18030 = Charset("GB18030")
)

func ConvertByte2String(byte []byte, charset Charset) string {

	var str string
	switch charset {
	case GB18030:
		var decodeBytes, _ = simplifiedchinese.GB18030.NewDecoder().Bytes(byte)
		str = string(decodeBytes)
	case UTF8:
		fallthrough
	default:
		str = string(byte)
	}

	return str
}

func BytesToBinaryString(bs []byte) string {
	return biu.BytesToBinaryString(bs)
}

func BinaryStringToBytes(str string) []byte {
	return biu.BinaryStringToBytes(str)
}

func WriteString(file string, data string) error {

	dir := path.Dir(file)
	if !Exists(dir) {
		err := os.MkdirAll(dir, 0766)
		if err != nil {
			return err
		}
	}
	openFile, err := os.OpenFile(file, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0766)
	if err != nil {
		return err
	}

	_, err = openFile.WriteString(data)
	if err != nil {
		return err
	}
	defer openFile.Close()
	return nil
}

/**
* 驼峰命名转下划线命名
* 思路:
* 小写和大写紧挨一起的地方,加上分隔符,然后全部转小写
 */
func UnCamelize(str string) string {
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	submatch := re.FindAllStringSubmatch(str, -1)
	for _, item := range submatch {
		str = strings.Replace(str, item[0], item[1]+"_"+item[2], -1)
	}
	str = strings.ToLower(str)
	return str
}

/**
* 下划线转驼峰
 */
func Camelize(str string) string {
	split := strings.Split(strings.ToLower(str), "_")
	var newStr string
	for _, item := range split {
		newStr += UcFirst(item)
	}
	return newStr
}

/**
* 字符串首字母转化为大写
 */
func UcFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if IsLetterLower(s[0]) {
		return string(s[0]-32) + s[1:]
	}
	return s
}

func IsLetterLower(b byte) bool {
	if b >= byte('a') && b <= byte('z') {
		return true
	}
	return false
}

// 判断所给路径文件/文件夹是否存在
func Exists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func GetComments(comments typemap.DefinitionComments) []string {
	text := strings.TrimSuffix(comments.Leading, "\n")
	if len(strings.TrimSpace(text)) == 0 {
		return nil
	}
	split := strings.Split(text, "\n")
	return split
}

func GetThinkPath(path string) (string, error) {
	c := 0
LOOP:
	thinkFile := path + "/start.php"
	if Exists(thinkFile) {
		return path, nil
	} else {
		if c > 10 {
			goto ERROR
		}
		path = filepath.Dir(path)
		c++
		goto LOOP
	}
ERROR:
	return "", errors.New("proto file dir is wrong")
}

func Isset(s []string, index int) bool {
	for idx, _ := range s {
		if idx == index {
			return true
		}
	}
	return false
}

func TrimRule(str string) string {
	if "" == str {
		return str
	}
	reg, _ := regexp.Compile("`(.*)`")
	str = reg.ReplaceAllString(str, "")
	return strings.Trim(str, "\n\r ")
}

type Parameter struct {
	params map[string]string
}

func (p *Parameter) Get(name string, defaults ...string) string {
	if val, ok := p.params[name]; ok {
		return val
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

func GetParameter(parameter string) *Parameter {
	return &Parameter{params: ParseParameter(parameter)}
}

func ParseParameter(parameter string) map[string]string {
	maps := make(map[string]string)
	if parameter == "" {
		return maps
	}
	split := strings.Split(parameter, ",")
	for _, s := range split {
		exp := strings.Split(s, "=")
		maps[exp[0]] = exp[1]
	}
	return maps
}
