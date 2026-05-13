package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// HandleValidation 处理 op=13 场景，返回 plain_token 和 signature。
func HandleValidation(body map[string]any, botSecret string) (map[string]any, error) {
	d, ok := body["d"].(map[string]any)
	if !ok {
		return nil, errors.New("无效的传参：缺少d")
	}

	eventTS, ok := d["event_ts"].(string)
	if !ok {
		return nil, errors.New("无效的传参：缺少d.event_ts")
	}

	plainToken, ok := d["plain_token"].(string)
	if !ok {
		return nil, errors.New("无效的传参：缺少d.plain_token")
	}

	seedBytes := []byte(botSecret)
	if len(seedBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("无效的bot secret 长度：需要 %d, 但取得 %d", ed25519.SeedSize, len(seedBytes))
	}

	privateKey := ed25519.NewKeyFromSeed(seedBytes)
	msgBytes := []byte(eventTS + plainToken)
	signature := ed25519.Sign(privateKey, msgBytes)

	return map[string]any{
		"plain_token": plainToken,
		"signature":   hex.EncodeToString(signature),
	}, nil
}

// VerifySignature 从请求头解析并基础校验签名。
func VerifySignature(headers map[string]string) (bool, []byte) {
	signatureHex := headers["X-Signature-Ed25519"]
	if signatureHex == "" {
		return false, nil
	}

	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, nil
	}

	if len(sig) != ed25519.SignatureSize || (sig[63]&224) != 0 {
		return false, nil
	}

	return true, sig
}

// GenerateKeys 兼容 Python 版本的 seed 填充逻辑（重复直到 32 字节）。
func GenerateKeys(seedBytes []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if len(seedBytes) == 0 {
		return nil, nil, errors.New("seed为空")
	}

	seed := append([]byte(nil), seedBytes...)
	for len(seed) < ed25519.SeedSize {
		seed = append(seed, seed...)
		if len(seed) > ed25519.SeedSize {
			seed = seed[:ed25519.SeedSize]
		}
	}
	if len(seed) > ed25519.SeedSize {
		seed = seed[:ed25519.SeedSize]
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return publicKey, privateKey, nil
}

// GetSignatureBody 返回待验签消息：timestamp + body。
func GetSignatureBody(headers map[string]string, body []byte) (bool, []byte) {
	timestamp := headers["X-Signature-Timestamp"]
	if timestamp == "" {
		return false, nil
	}

	msg := append([]byte(timestamp), body...)
	return true, msg
}

// HandleValidationWebhook 执行 webhook 场景签名校验。
func HandleValidationWebhook(headers map[string]string, body []byte, bodySecret string) ([]byte, error) {
	success, sig := VerifySignature(headers)
	if !success {
		return nil, errors.New("验签不成功")
	}

	publicKey, _, err := GenerateKeys([]byte(bodySecret))
	if err != nil {
		return nil, err
	}

	success, msg := GetSignatureBody(headers, body)
	if !success {
		return nil, errors.New("获取签名body不成功")
	}

	if !ed25519.Verify(publicKey, msg, sig) {
		return nil, errors.New("签名验证失败")
	}

	return msg, nil
}

// Verify 是统一入口，返回与 Python 版本一致结构的数据。
func Verify(evt map[string]any, botSecret string) map[string]any {
	headersAny, ok := evt["headers"].(map[string]any)
	if !ok {
		return map[string]any{"code": 403, "msg": "无效的请求头"}
	}

	headers := make(map[string]string, len(headersAny))
	for k, v := range headersAny {
		headers[k] = fmt.Sprint(v)
	}

	if headers["User-Agent"] != "QQBot-Callback" {
		return map[string]any{"code": 403, "msg": "请求头不来自QQBot"}
	}

	if headers["X-Signature-Ed25519"] == "" {
		return map[string]any{"code": 403, "msg": "签名不在请求头中"}
	}

	if headers["X-Signature-Timestamp"] == "" {
		return map[string]any{"code": 403, "msg": "时间戳不在请求头中"}
	}

	bodyStr, ok := evt["body"].(string)
	if !ok {
		return map[string]any{"code": 403, "msg": "无效的请求体"}
	}

	bytesBody := []byte(bodyStr)
	var body map[string]any
	if err := json.Unmarshal(bytesBody, &body); err != nil {
		return map[string]any{"code": 403, "msg": "无效的json请求体"}
	}

	op, _ := body["op"].(float64)
	if int(op) == 13 {
		resp, err := HandleValidation(body, botSecret)
		if err != nil {
			return map[string]any{"code": 403, "msg": err.Error()}
		}
		return resp
	}

	_, err := HandleValidationWebhook(headers, bytesBody, botSecret)
	if err != nil {
		return map[string]any{"code": 403, "msg": err.Error()}
	}

	return map[string]any{"op": 12}
}
