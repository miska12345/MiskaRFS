// package message implements a generic-network-response packet
package message

import "encoding/json"

const TYPE_RESPONSE = "text/res"
const TYPE_ERROR = "text/error"

type Message struct {
	Type string
	Msg  string
}

func New(msgType, msg string) *Message {
	return &Message{
		Type: msgType,
		Msg:  msg,
	}
}

func (g *Message) ConvertToNetForm() ([]byte, error) {
	j, err := json.Marshal(g)
	if err != nil {
		return j, err
	}
	return j, nil
}

func ConvertFromNetForm(bs []byte) (res *Message, err error) {
	res = new(Message)
	err = json.Unmarshal(bs, res)
	if err != nil {
		return
	}
	return
}