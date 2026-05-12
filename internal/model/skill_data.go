package model

// SkillData 카드 스킬의 도메인 표현.
// model이 protobuf에 의존하지 않도록 pb.SkillData를 미러링한다.
// JSON 필드명은 pb.SkillData와 동일하게 유지하여 DB JSON 컬럼 호환성 보존.
// handler/card/convert.go가 pb.SkillData와의 양방향 변환을 담당한다.
type SkillData struct {
	Exp     uint32 `json:"exp,omitempty"`
	Slot    uint32 `json:"slot,omitempty"`
	Level   uint32 `json:"level,omitempty"`
	SkillID uint32 `json:"skill_id,omitempty"`
}
