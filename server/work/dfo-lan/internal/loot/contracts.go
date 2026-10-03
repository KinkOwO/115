package loot

// ContractResolver resolves the premium activation carried by an item. The
// workflow supplies the provider without exposing cashshop types to loot.
type ContractResolver func(uint32) (PremiumActivation, bool)
