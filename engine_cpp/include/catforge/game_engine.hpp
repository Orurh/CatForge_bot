#pragma once

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

namespace catforge::engine {

inline constexpr std::uint32_t kCurrentRulesVersion = 14;
inline constexpr std::uint32_t kCurrentContentVersion = 5;
inline constexpr std::int32_t kEnergyMax = 100;
inline constexpr std::int32_t kTrainingMinEnergy = 50;
inline constexpr std::int64_t kEnergyRegenIntervalNanos = 480'000'000'000;

enum class TrainingOutcome {
  kOk,
  kNotEnoughEnergy,
};

enum class Encounter {
  kUnspecified,
  kMicePack,
  kPigeon,
  kLizard,
  kBigRat,
};

struct StatDelta {
  std::int32_t hp{};
  std::int32_t atk{};
  std::int32_t def{};
  std::int32_t spd{};

  bool operator==(const StatDelta &) const = default;
};

struct FelineStats {
  std::int32_t claws_tenth_mm{}, weight_grams{}, tail_mm{}, whisker_span_mm{};
  bool operator==(const FelineStats &) const = default;
};
struct CatState {
  std::int64_t id{};
  std::int64_t user_id{};
  std::int64_t state_version{};
  std::string name;
  std::string breed;
  std::string trait;
  std::int32_t level{1};
  std::int64_t xp{};
  std::int64_t coins{};
  std::int32_t energy{kEnergyMax};
  std::optional<std::int64_t> last_train_at_unix_nanos;
  std::optional<std::int64_t> energy_updated_at_unix_nanos;
  std::int32_t hp_base{};
  std::int32_t atk_base{};
  std::int32_t def_base{};
  std::int32_t spd_base{};
  FelineStats feline;
  bool first_item_granted{};
};

struct TrainingRandom {
  std::int32_t training_crit_roll{};
  std::int32_t energy_cost_roll{};
  std::int32_t xp_gain_roll{};
  std::int32_t encounter_roll{};
  std::uint32_t flavor_roll{};
  std::int32_t training_loot_roll{};
};

struct TrainingInput {
  std::uint32_t rules_version{kCurrentRulesVersion};
  std::uint32_t content_version{kCurrentContentVersion};
  CatState cat;
  std::int64_t now_unix_nanos{};
  TrainingRandom random;
  std::int32_t training_crit_bonus_percent{};
};

struct TrainResult {
  std::string loot_item_id;
  std::vector<std::string> progression_facts;
  TrainingOutcome outcome{TrainingOutcome::kOk};
  std::int64_t xp_gain{};
  std::int32_t energy_cost{};
  bool crit{};
  std::int32_t efficiency_percent{};
  std::int32_t levels_gained{};
  StatDelta stats_gained;
  Encounter encounter{Encounter::kUnspecified};
  std::uint32_t flavor{};
  std::int64_t coins_gain{};
};

struct TrainingOutput {
  CatState cat;
  TrainResult result;
};

enum class ExpeditionOutcome { kVictory, kDefeat, kNotEnoughEnergy };
enum class ExpeditionLocation { kUnspecified, kAlley, kRooftop, kPark };
enum class ExpeditionDifficulty { kUnspecified, kEasy, kNormal, kHard };
enum class EnemyKind {
  kUnspecified,
  kSewerRat,
  kStrayDog,
  kWildLynx,
  kRatAccountant,
  kCourierDog,
  kMoonLynx
};
enum class BattleActor { kCat, kEnemy };
enum class ItemRarity { kUnspecified, kCommon, kRare, kEpic };

struct Enemy {
  EnemyKind kind{EnemyKind::kUnspecified};
  std::int32_t level{};
  std::int32_t hp{};
  std::int32_t atk{};
  std::int32_t def{};
  std::int32_t spd{};

  bool operator==(const Enemy &) const = default;
};

struct BattleTurn {
  std::int32_t round{};
  BattleActor actor{BattleActor::kCat};
  std::int32_t damage{};
  bool crit{};
  std::int32_t defender_hp_after{};

  bool operator==(const BattleTurn &) const = default;
};

struct ExpeditionInput {
  std::uint32_t rules_version{kCurrentRulesVersion};
  std::uint32_t content_version{kCurrentContentVersion};
  CatState cat;
  std::int64_t now_unix_nanos{};
  std::uint64_t seed{};
  ExpeditionLocation location{ExpeditionLocation::kUnspecified};
  ExpeditionDifficulty difficulty{ExpeditionDifficulty::kUnspecified};
  StatDelta equipment_bonus;
  struct LootCounts {
    std::int32_t common{};
    std::int32_t rare{};
    std::int32_t epic{};
  } loot_counts;
  bool force_loot{};
};

struct LootRoll {
  bool dropped{};
  ItemRarity rarity{ItemRarity::kUnspecified};
  std::int32_t item_index{};

  bool operator==(const LootRoll &) const = default;
};

struct ExpeditionResult {
  ExpeditionOutcome outcome{ExpeditionOutcome::kVictory};
  ExpeditionLocation location{ExpeditionLocation::kUnspecified};
  ExpeditionDifficulty difficulty{ExpeditionDifficulty::kUnspecified};
  Enemy enemy;
  std::int32_t energy_cost{};
  std::int64_t xp_gain{};
  std::int64_t coins_gain{};
  std::int32_t rounds{};
  std::int32_t cat_hp_after{};
  std::int32_t enemy_hp_after{};
  std::int32_t levels_gained{};
  StatDelta stats_gained;
  std::vector<BattleTurn> turns;
  LootRoll loot;
};

struct ExpeditionOutput {
  CatState cat;
  ExpeditionResult result;
};

enum class YardEventType { kUnspecified, kFishTruck, kBigDog, kBigBox };
enum class YardEventChoice { kUnspecified, kSteal, kDistract, kScout };
enum class YardEventOutcomeTier {
  kFailure,
  kPartial,
  kSuccess,
  kExceptional,
};

struct YardEventParticipant {
  std::int64_t cat_id{};
  YardEventChoice choice{YardEventChoice::kUnspecified};
  std::int32_t level{1};
  std::int32_t hp{};
  std::int32_t atk{};
  std::int32_t def{};
  std::int32_t spd{};
  FelineStats feline;
  std::vector<std::string> effects;
  std::string special_action;
};

struct YardEventInput {
  std::uint32_t rules_version{kCurrentRulesVersion};
  std::uint32_t content_version{kCurrentContentVersion};
  std::uint64_t seed{};
  YardEventType event_type{YardEventType::kUnspecified};
  std::vector<YardEventParticipant> participants;
};

struct YardEventParticipantResult {
  std::int64_t cat_id{};
  YardEventChoice choice{YardEventChoice::kUnspecified};
  std::int32_t contribution{};
  bool mvp{};

  bool item_effect_triggered{};

  bool operator==(const YardEventParticipantResult &) const = default;
};

struct YardRelationshipEffect {
  std::int64_t cat_a_id{};
  std::int64_t cat_b_id{};
  std::int32_t friendship_delta{};
  std::int32_t rivalry_delta{};
  std::int32_t respect_delta{};

  bool operator==(const YardRelationshipEffect &) const = default;
};

struct YardEventResult {
  YardEventOutcomeTier outcome_tier{YardEventOutcomeTier::kFailure};
  std::int32_t team_score{};
  std::int32_t target_score{};
  std::int32_t yard_score{};
  std::int64_t xp_gain{};
  bool secret_found{};
  std::int32_t strategy_bonus{};
  std::vector<YardEventParticipantResult> participants;
  std::vector<YardRelationshipEffect> relationship_effects;

  bool operator==(const YardEventResult &) const = default;
};

struct FightInput {
  std::uint32_t rules_version{kCurrentRulesVersion};
  std::uint32_t content_version{kCurrentContentVersion};
  std::uint64_t seed{};
  CatState cat_a;
  CatState cat_b;
};

struct FightTurn {
  std::int32_t round{};
  std::int64_t attacker_cat_id{};
  std::int64_t defender_cat_id{};
  std::int32_t damage{};
  bool crit{};
  std::int32_t defender_hp_after{};

  bool operator==(const FightTurn &) const = default;
};

struct FightResult {
  std::int64_t winner_cat_id{};
  std::int64_t loser_cat_id{};
  std::int32_t rounds{};
  std::int32_t final_hp_a{};
  std::int32_t final_hp_b{};
  std::vector<FightTurn> turns;

  bool operator==(const FightResult &) const = default;
};

struct ProgressInput {
  CatState cat;
  std::int64_t xp_gain{};
  std::uint64_t seed{};
  std::string source;
  std::string outcome_tier;
  bool secret_found{};
  std::int32_t energy_spent{};
  std::int32_t training_loot_roll{};
};
struct ProgressOutput {
  CatState cat;
  std::string loot_item_id;
  std::vector<std::string> facts;
};
FelineStats BaseFelineStats(const std::string &breed);
FelineStats FelineGrowth(const std::string &breed);
ProgressOutput Progress(const ProgressInput &input);
TrainingOutput Train(const TrainingInput &input);
ExpeditionOutput Expedition(const ExpeditionInput &input);
YardEventResult ResolveYardEvent(const YardEventInput &input);
FightResult Fight(const FightInput &input);

} // namespace catforge::engine
