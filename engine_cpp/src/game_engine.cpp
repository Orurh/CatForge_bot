#include "catforge/game_engine.hpp"

#include <algorithm>
#include <stdexcept>
#include <utility>

namespace catforge::engine {
namespace {

std::int32_t RegenEnergy(const CatState &cat, const std::int64_t now) {
  if (cat.energy >= kEnergyMax ||
      !cat.energy_updated_at_unix_nanos.has_value()) {
    return std::min(cat.energy, kEnergyMax);
  }
  const auto delta = now - *cat.energy_updated_at_unix_nanos;
  if (delta <= 0) {
    return cat.energy;
  }
  const auto regenerated = delta / kEnergyRegenIntervalNanos;
  return static_cast<std::int32_t>(std::min<std::int64_t>(
      static_cast<std::int64_t>(cat.energy) + regenerated, kEnergyMax));
}

std::pair<double, std::int32_t>
TrainingEfficiency(const std::optional<std::int64_t> last_train_at,
                   const std::int64_t now) {
  if (!last_train_at.has_value()) {
    return {1.0, 100};
  }
  const auto delta = now - *last_train_at;
  if (delta <= 0) {
    return {kMinimumEfficiency, 50};
  }

  double efficiency = 1.0;
  if (delta < kEfficiencyWindowNanos) {
    const auto ratio = static_cast<double>(delta) /
                       static_cast<double>(kEfficiencyWindowNanos);
    efficiency = kMinimumEfficiency + (1.0 - kMinimumEfficiency) * ratio;
    efficiency = std::max(efficiency, kMinimumEfficiency);
  }
  return {efficiency, static_cast<std::int32_t>(efficiency * 100.0 + 0.5)};
}

std::int32_t TrainingEnergyCost(const std::int32_t energy) {
  if (energy < kTrainingMinEnergy) {
    return 0;
  }
  return std::clamp(energy - kTrainingEnergyReserve, kTrainingMinEnergy,
                    kTrainingMaxEnergyCost);
}

Encounter EncounterFromRoll(const std::int32_t roll) {
  if (roll < 10) {
    return Encounter::kBigRat;
  }
  if (roll < 45) {
    return Encounter::kMicePack;
  }
  if (roll < 70) {
    return Encounter::kPigeon;
  }
  return Encounter::kLizard;
}

StatDelta LevelUpDelta(const std::string &breed) {
  if (breed == "maine_coon") {
    return {.hp = 4, .atk = 2, .def = 2, .spd = 1};
  }
  if (breed == "siamese") {
    return {.hp = 4, .atk = 2, .def = 1, .spd = 2};
  }
  if (breed == "british") {
    return {.hp = 3, .atk = 2, .def = 2, .spd = 1};
  }
  if (breed == "bengal") {
    return {.hp = 3, .atk = 2, .def = 1, .spd = 2};
  }
  return {.hp = 3, .atk = 2, .def = 2, .spd = 2};
}

void Validate(const TrainingInput &input) {
  if (input.rules_version != kCurrentRulesVersion) {
    throw std::invalid_argument("unsupported rules version");
  }
  if (input.content_version != kCurrentContentVersion) {
    throw std::invalid_argument("unsupported content version");
  }
  if (input.cat.level < 1) {
    throw std::invalid_argument("cat level must be positive");
  }
  if (input.random.energy_cost_roll < 0 ||
      input.random.energy_cost_roll >= 13 || input.random.xp_gain_roll < 0 ||
      input.random.xp_gain_roll >= 17 || input.random.encounter_roll < 0 ||
      input.random.encounter_roll >= 100 ||
      input.random.flavor_roll >= (1U << 16U)) {
    throw std::invalid_argument("random roll outside its allowed range");
  }
}

class SplitMix64 {
public:
  explicit SplitMix64(const std::uint64_t seed) : state_(seed) {}

  std::uint64_t Next() {
    state_ += 0x9e3779b97f4a7c15ULL;
    auto z = state_;
    z = (z ^ (z >> 30U)) * 0xbf58476d1ce4e5b9ULL;
    z = (z ^ (z >> 27U)) * 0x94d049bb133111ebULL;
    return z ^ (z >> 31U);
  }

private:
  std::uint64_t state_;
};

std::int32_t ExpeditionEnergyCost(const ExpeditionDifficulty difficulty) {
  switch (difficulty) {
  case ExpeditionDifficulty::kEasy:
    return 20;
  case ExpeditionDifficulty::kNormal:
    return 30;
  case ExpeditionDifficulty::kHard:
    return 40;
  default:
    return 0;
  }
}

Enemy GenerateEnemy(const std::int32_t cat_level,
                    const ExpeditionLocation location,
                    const ExpeditionDifficulty difficulty, SplitMix64 &rng) {
  const auto difficulty_offset = difficulty == ExpeditionDifficulty::kEasy ? -1
                                 : difficulty == ExpeditionDifficulty::kHard
                                     ? 1
                                     : 0;
  const auto level = std::max<std::int32_t>(
      1, cat_level + static_cast<std::int32_t>(rng.Next() % 3U) - 1 +
             difficulty_offset);
  const auto rare = rng.Next() % 100U < 5U;
  const auto combat_level = level;
  Enemy enemy;
  if (location == ExpeditionLocation::kAlley) {
    enemy = {.kind = EnemyKind::kSewerRat,
             .level = level,
             .hp = 40 + combat_level * 3,
             .atk = 13 + combat_level * 3 / 2,
             .def = 14 + combat_level * 17 / 10,
             .spd = 12 + combat_level * 3 / 2};
    if (rare) {
      enemy.kind = EnemyKind::kRatAccountant;
      enemy.hp = enemy.hp * 115 / 100;
      enemy.atk = enemy.atk * 105 / 100;
      enemy.def = enemy.def * 110 / 100;
    }
  } else if (location == ExpeditionLocation::kRooftop) {
    enemy = {.kind = EnemyKind::kStrayDog,
             .level = level,
             .hp = 46 + combat_level * 7 / 2,
             .atk = 13 + combat_level * 7 / 5,
             .def = 16 + combat_level * 17 / 10,
             .spd = 18 + combat_level * 17 / 10};
    if (rare) {
      enemy.kind = EnemyKind::kCourierDog;
      enemy.hp = enemy.hp * 110 / 100;
      enemy.atk = enemy.atk * 108 / 100;
      enemy.spd = enemy.spd * 115 / 100;
    }
  } else {
    enemy = {.kind = EnemyKind::kWildLynx,
             .level = level,
             .hp = 32 + combat_level * 3,
             .atk = 17 + combat_level * 17 / 10,
             .def = 10 + combat_level * 6 / 5,
             .spd = 18 + combat_level * 17 / 10};
    if (rare) {
      enemy.kind = EnemyKind::kMoonLynx;
      enemy.hp = enemy.hp * 110 / 100;
      enemy.atk = enemy.atk * 112 / 100;
      enemy.spd = enemy.spd * 108 / 100;
    }
  }
  if (difficulty == ExpeditionDifficulty::kEasy) {
    enemy.hp = std::max(1, enemy.hp * 108 / 100);
    enemy.atk = std::max(1, enemy.atk * 108 / 100);
    enemy.def = std::max(1, enemy.def * 104 / 100);
  } else if (difficulty == ExpeditionDifficulty::kNormal) {
    enemy.hp = std::max(1, enemy.hp * 112 / 100);
    enemy.atk = std::max(1, enemy.atk * 112 / 100);
    enemy.def = std::max(1, enemy.def * 105 / 100);
  } else if (difficulty == ExpeditionDifficulty::kHard) {
    enemy.hp = std::max(1, enemy.hp * 112 / 100);
    enemy.atk = std::max(1, enemy.atk * 112 / 100);
    enemy.def = std::max(1, enemy.def * 105 / 100);
  }
  return enemy;
}

std::pair<std::int64_t, std::int64_t>
ExpeditionReward(const ExpeditionDifficulty difficulty,
                 const std::int32_t enemy_level, const EnemyKind enemy_kind) {
  std::pair<std::int64_t, std::int64_t> reward;
  if (difficulty == ExpeditionDifficulty::kEasy) {
    reward = {20 + enemy_level * 6, 8 + enemy_level * 2};
  } else if (difficulty == ExpeditionDifficulty::kHard) {
    reward = {55 + enemy_level * 10, 25 + enemy_level * 4};
  } else {
    reward = {35 + enemy_level * 8, 15 + enemy_level * 3};
  }
  if (enemy_kind == EnemyKind::kRatAccountant ||
      enemy_kind == EnemyKind::kCourierDog ||
      enemy_kind == EnemyKind::kMoonLynx) {
    reward.first = reward.first * 3 / 2;
    reward.second = reward.second * 3 / 2;
  }
  return reward;
}

std::pair<std::int32_t, bool> RollDamage(const std::int32_t attack,
                                         const std::int32_t defense,
                                         const std::int32_t crit_chance,
                                         SplitMix64 &rng) {
  auto damage = std::max<std::int32_t>(
      1, attack - defense / 2 + static_cast<std::int32_t>(rng.Next() % 5U) - 2);
  const auto critical =
      rng.Next() % 100U < static_cast<std::uint64_t>(crit_chance);
  if (critical) {
    damage *= 2;
  }
  return {damage, critical};
}

void ApplyLevelUps(CatState &cat, std::int32_t &levels, StatDelta &gained) {
  while (cat.xp >= static_cast<std::int64_t>(cat.level) * 100) {
    cat.xp -= static_cast<std::int64_t>(cat.level) * 100;
    ++cat.level;
    ++levels;
    const auto delta = LevelUpDelta(cat.breed);
    cat.hp_base += delta.hp;
    cat.atk_base += delta.atk;
    cat.def_base += delta.def;
    cat.spd_base += delta.spd;
    gained.hp += delta.hp;
    gained.atk += delta.atk;
    gained.def += delta.def;
    gained.spd += delta.spd;
  }
}

LootRoll RollLoot(const ExpeditionDifficulty difficulty,
                  const ExpeditionInput::LootCounts counts, const bool force,
                  SplitMix64 &rng) {
  auto chest_chance = 40;
  auto common_weight = 70;
  auto rare_weight = 28;
  auto epic_weight = 2;
  if (difficulty == ExpeditionDifficulty::kEasy) {
    chest_chance = 20;
    common_weight = 90;
    rare_weight = 10;
    epic_weight = 0;
  } else if (difficulty == ExpeditionDifficulty::kHard) {
    chest_chance = 70;
    common_weight = 45;
    rare_weight = 48;
    epic_weight = 7;
  }
  const auto chest_roll = rng.Next() % 100U;
  if (counts.common + counts.rare + counts.epic <= 0 ||
      (!force && chest_roll >= static_cast<std::uint64_t>(chest_chance))) {
    return {};
  }
  if (counts.common <= 0)
    common_weight = 0;
  if (counts.rare <= 0)
    rare_weight = 0;
  if (counts.epic <= 0)
    epic_weight = 0;
  const auto total = common_weight + rare_weight + epic_weight;
  if (total <= 0)
    return {};
  const auto roll = static_cast<std::int32_t>(rng.Next() % total);
  auto rarity = ItemRarity::kCommon;
  auto count = counts.common;
  if (roll >= common_weight) {
    rarity = ItemRarity::kRare;
    count = counts.rare;
    if (roll >= common_weight + rare_weight) {
      rarity = ItemRarity::kEpic;
      count = counts.epic;
    }
  }
  return {.dropped = true,
          .rarity = rarity,
          .item_index = static_cast<std::int32_t>(rng.Next() % count)};
}

} // namespace

TrainingOutput Train(const TrainingInput &input) {
  Validate(input);
  auto cat = input.cat;
  cat.energy = RegenEnergy(cat, input.now_unix_nanos);
  cat.energy_updated_at_unix_nanos = input.now_unix_nanos;

  const auto cost = TrainingEnergyCost(cat.energy);
  if (cost == 0) {
    return {
        .cat = std::move(cat),
        .result = {.outcome = TrainingOutcome::kNotEnoughEnergy,
                   .efficiency_percent = 100},
    };
  }

  auto gain_base =
      static_cast<double>(cost * 3 / 2 + input.random.xp_gain_roll) +
      static_cast<double>(cat.level / 2);
  const auto [efficiency, efficiency_percent] =
      TrainingEfficiency(cat.last_train_at_unix_nanos, input.now_unix_nanos);
  const auto encounter = EncounterFromRoll(input.random.encounter_roll);
  const auto critical = encounter == Encounter::kBigRat;
  if (critical) {
    gain_base *= 2.0;
  }
  const auto gain = std::max<std::int64_t>(
      static_cast<std::int64_t>(gain_base * efficiency), 1);
  const auto coins_gain = std::max<std::int64_t>(cost / 10, 1);

  cat.energy -= cost;
  cat.xp += gain;
  cat.coins += coins_gain;
  std::int32_t levels_gained = 0;
  StatDelta stats_gained;
  while (cat.xp >= static_cast<std::int64_t>(cat.level) * 100) {
    cat.xp -= static_cast<std::int64_t>(cat.level) * 100;
    ++cat.level;
    ++levels_gained;
    const auto delta = LevelUpDelta(cat.breed);
    cat.hp_base += delta.hp;
    cat.atk_base += delta.atk;
    cat.def_base += delta.def;
    cat.spd_base += delta.spd;
    stats_gained.hp += delta.hp;
    stats_gained.atk += delta.atk;
    stats_gained.def += delta.def;
    stats_gained.spd += delta.spd;
  }
  cat.last_train_at_unix_nanos = input.now_unix_nanos;

  return {
      .cat = std::move(cat),
      .result =
          {
              .outcome = TrainingOutcome::kOk,
              .xp_gain = gain,
              .energy_cost = cost,
              .crit = critical,
              .efficiency_percent = efficiency_percent,
              .levels_gained = levels_gained,
              .stats_gained = stats_gained,
              .encounter = encounter,
              .flavor = input.random.flavor_roll,
              .coins_gain = coins_gain,
          },
  };
}

ExpeditionOutput Expedition(const ExpeditionInput &input) {
  if (input.rules_version != kCurrentRulesVersion) {
    throw std::invalid_argument("unsupported rules version");
  }
  if (input.content_version != kCurrentContentVersion) {
    throw std::invalid_argument("unsupported content version");
  }
  if (input.cat.level < 1) {
    throw std::invalid_argument("cat level must be positive");
  }
  const auto energy_cost = ExpeditionEnergyCost(input.difficulty);
  if (energy_cost == 0 || (input.location != ExpeditionLocation::kAlley &&
                           input.location != ExpeditionLocation::kRooftop &&
                           input.location != ExpeditionLocation::kPark)) {
    throw std::invalid_argument("invalid expedition location or difficulty");
  }

  auto cat = input.cat;
  cat.energy = RegenEnergy(cat, input.now_unix_nanos);
  cat.energy_updated_at_unix_nanos = input.now_unix_nanos;
  if (cat.energy < energy_cost) {
    return {.cat = std::move(cat),
            .result = {.outcome = ExpeditionOutcome::kNotEnoughEnergy,
                       .location = input.location,
                       .difficulty = input.difficulty}};
  }
  cat.energy -= energy_cost;

  SplitMix64 rng(input.seed);
  const auto enemy =
      GenerateEnemy(cat.level, input.location, input.difficulty, rng);
  auto cat_hp = std::max(1, cat.hp_base + input.equipment_bonus.hp);
  auto enemy_hp = enemy.hp;
  const auto cat_atk = std::max(1, cat.atk_base + input.equipment_bonus.atk);
  const auto cat_def = std::max(0, cat.def_base + input.equipment_bonus.def);
  const auto cat_spd = std::max(0, cat.spd_base + input.equipment_bonus.spd);
  const auto cat_first =
      cat_spd > enemy.spd || (cat_spd == enemy.spd && rng.Next() % 2U == 0U);
  std::vector<BattleTurn> turns;
  turns.reserve(20);
  std::int32_t rounds = 0;

  const auto attack = [&](const std::int32_t round, const BattleActor actor,
                          const std::int32_t atk, const std::int32_t def,
                          const std::int32_t crit_chance,
                          std::int32_t &defender_hp) {
    const auto [damage, critical] = RollDamage(atk, def, crit_chance, rng);
    defender_hp = std::max(0, defender_hp - damage);
    turns.push_back({.round = round,
                     .actor = actor,
                     .damage = damage,
                     .crit = critical,
                     .defender_hp_after = defender_hp});
  };

  for (std::int32_t round = 1; round <= 100 && cat_hp > 0 && enemy_hp > 0;
       ++round) {
    rounds = round;
    if (cat_first) {
      attack(round, BattleActor::kCat, cat_atk, enemy.def, 12, enemy_hp);
      if (enemy_hp == 0) {
        break;
      }
      attack(round, BattleActor::kEnemy, enemy.atk, cat_def, 8, cat_hp);
    } else {
      attack(round, BattleActor::kEnemy, enemy.atk, cat_def, 8, cat_hp);
      if (cat_hp == 0) {
        break;
      }
      attack(round, BattleActor::kCat, cat_atk, enemy.def, 12, enemy_hp);
    }
  }

  const auto victory = cat_hp > 0 && enemy_hp == 0;
  const auto [victory_xp, victory_coins] =
      ExpeditionReward(input.difficulty, enemy.level, enemy.kind);
  const auto xp_gain = victory ? victory_xp : 5;
  const auto coins_gain = victory ? victory_coins : 0;
  cat.xp += xp_gain;
  cat.coins += coins_gain;
  std::int32_t levels_gained = 0;
  StatDelta stats_gained;
  ApplyLevelUps(cat, levels_gained, stats_gained);
  const auto loot = victory ? RollLoot(input.difficulty, input.loot_counts,
                                       input.force_loot, rng)
                            : LootRoll{};

  return {.cat = std::move(cat),
          .result = {.outcome = victory ? ExpeditionOutcome::kVictory
                                        : ExpeditionOutcome::kDefeat,
                     .location = input.location,
                     .difficulty = input.difficulty,
                     .enemy = enemy,
                     .energy_cost = energy_cost,
                     .xp_gain = xp_gain,
                     .coins_gain = coins_gain,
                     .rounds = rounds,
                     .cat_hp_after = cat_hp,
                     .enemy_hp_after = enemy_hp,
                     .levels_gained = levels_gained,
                     .stats_gained = stats_gained,
                     .turns = std::move(turns),
                     .loot = loot}};
}

FightResult Fight(const FightInput &input) {
  if (input.rules_version != kCurrentRulesVersion) {
    throw std::invalid_argument("unsupported rules version");
  }
  if (input.content_version != kCurrentContentVersion) {
    throw std::invalid_argument("unsupported content version");
  }
  const auto valid_cat = [](const CatState &cat) {
    return cat.id > 0 && cat.level > 0 && cat.hp_base > 0 && cat.atk_base > 0 &&
           cat.def_base >= 0 && cat.spd_base >= 0;
  };
  if (!valid_cat(input.cat_a) || !valid_cat(input.cat_b) ||
      input.cat_a.id == input.cat_b.id) {
    throw std::invalid_argument("fight requires two distinct valid cats");
  }

  SplitMix64 rng(input.seed);
  auto hp_a = input.cat_a.hp_base;
  auto hp_b = input.cat_b.hp_base;
  auto a_starts = input.cat_a.spd_base > input.cat_b.spd_base;
  if (input.cat_a.spd_base == input.cat_b.spd_base) {
    a_starts = rng.Next() % 2U == 0U;
  }

  std::vector<FightTurn> turns;
  turns.reserve(32);
  std::int32_t rounds = 0;
  const auto strike = [&](const bool a_attacks, const std::int32_t round) {
    const auto &attacker = a_attacks ? input.cat_a : input.cat_b;
    const auto &defender = a_attacks ? input.cat_b : input.cat_a;
    auto &defender_hp = a_attacks ? hp_b : hp_a;
    const auto crit_chance =
        std::clamp(7 + (attacker.spd_base - defender.spd_base) / 3, 5, 20);
    const auto [damage, critical] =
        RollDamage(attacker.atk_base, defender.def_base, crit_chance, rng);
    defender_hp = std::max(0, defender_hp - damage);
    turns.push_back({.round = round,
                     .attacker_cat_id = attacker.id,
                     .defender_cat_id = defender.id,
                     .damage = damage,
                     .crit = critical,
                     .defender_hp_after = defender_hp});
  };

  while (hp_a > 0 && hp_b > 0 && rounds < 100) {
    ++rounds;
    strike(a_starts, rounds);
    if (hp_a == 0 || hp_b == 0) {
      break;
    }
    strike(!a_starts, rounds);
  }

  bool a_wins = hp_a > hp_b;
  if (hp_a == hp_b) {
    a_wins = rng.Next() % 2U == 0U;
  }
  return {.winner_cat_id = a_wins ? input.cat_a.id : input.cat_b.id,
          .loser_cat_id = a_wins ? input.cat_b.id : input.cat_a.id,
          .rounds = rounds,
          .final_hp_a = hp_a,
          .final_hp_b = hp_b,
          .turns = std::move(turns)};
}

YardEventResult ResolveYardEvent(const YardEventInput &input) {
  if (input.rules_version != kCurrentRulesVersion) {
    throw std::invalid_argument("unsupported rules version");
  }
  if (input.content_version != kCurrentContentVersion) {
    throw std::invalid_argument("unsupported content version");
  }
  if (input.event_type != YardEventType::kFishTruck) {
    throw std::invalid_argument("unsupported yard event type");
  }
  if (input.participants.size() > 100U) {
    throw std::invalid_argument("yard event supports at most 100 participants");
  }

  auto participants = input.participants;
  std::sort(participants.begin(), participants.end(),
            [](const auto &left, const auto &right) {
              return left.cat_id < right.cat_id;
            });
  for (std::size_t index = 0; index < participants.size(); ++index) {
    const auto &cat = participants[index];
    if (cat.cat_id <= 0 || cat.level < 1 || cat.hp < 0 || cat.atk < 0 ||
        cat.def < 0 || cat.spd < 0 ||
        cat.choice == YardEventChoice::kUnspecified) {
      throw std::invalid_argument("invalid yard event participant");
    }
    if (index > 0 && participants[index - 1].cat_id == cat.cat_id) {
      throw std::invalid_argument("duplicate yard event participant");
    }
  }

  SplitMix64 rng(input.seed);
  std::int32_t steal_count = 0;
  std::int32_t distract_count = 0;
  std::int32_t scout_count = 0;
  std::vector<YardEventParticipantResult> outcomes;
  outcomes.reserve(participants.size());
  for (const auto &cat : participants) {
    std::int32_t role_stat = 0;
    switch (cat.choice) {
    case YardEventChoice::kSteal:
      role_stat = cat.atk;
      ++steal_count;
      break;
    case YardEventChoice::kDistract:
      role_stat = (cat.hp + cat.def) / 2;
      ++distract_count;
      break;
    case YardEventChoice::kScout:
      role_stat = cat.spd;
      ++scout_count;
      break;
    case YardEventChoice::kUnspecified:
      break;
    }
    const auto contribution = std::max<std::int32_t>(
        1, cat.level * 2 + role_stat / 4 +
               static_cast<std::int32_t>(rng.Next() % 7U));
    outcomes.push_back({.cat_id = cat.cat_id,
                        .choice = cat.choice,
                        .contribution = contribution});
  }

  const auto coordinated_pairs = std::min(steal_count, distract_count);
  const auto strategy_bonus = coordinated_pairs * 4 + scout_count * 2;
  auto team_score = strategy_bonus;
  for (const auto &outcome : outcomes) {
    team_score += outcome.contribution;
  }
  const auto participant_count = static_cast<std::int32_t>(outcomes.size());
  const auto target_score = 7 + participant_count * 9;
  const auto success = team_score >= target_score;

  if (outcomes.empty()) {
    return {.success = false,
            .team_score = 0,
            .target_score = target_score,
            .fish_total = 0,
            .secret_found = false,
            .strategy_bonus = 0};
  }

  const auto scout_power = [&] {
    std::int32_t total = 0;
    for (const auto &outcome : outcomes) {
      if (outcome.choice == YardEventChoice::kScout) {
        total += outcome.contribution;
      }
    }
    return total;
  }();
  const auto secret_chance =
      std::min<std::int32_t>(70, 10 + scout_count * 10 + scout_power);
  const auto secret_found =
      scout_count > 0 &&
      rng.Next() % 100U < static_cast<std::uint64_t>(secret_chance);
  auto fish_total = success ? participant_count * 4 + 6 +
                                  static_cast<std::int32_t>(rng.Next() % 5U)
                            : participant_count;
  if (secret_found) {
    fish_total += 3;
  }

  const auto best =
      std::max_element(outcomes.begin(), outcomes.end(),
                       [](const auto &left, const auto &right) {
                         if (left.contribution != right.contribution) {
                           return left.contribution < right.contribution;
                         }
                         return left.cat_id > right.cat_id;
                       });
  const auto equal_share = fish_total / participant_count;
  auto remainder = fish_total % participant_count;
  for (auto &outcome : outcomes) {
    outcome.fish_reward = equal_share;
    outcome.mvp = outcome.cat_id == best->cat_id;
    if (remainder > 0) {
      ++outcome.fish_reward;
      --remainder;
    }
  }

  std::vector<YardRelationshipEffect> relationship_effects;
  relationship_effects.reserve(outcomes.size() * (outcomes.size() - 1U) / 2U);
  for (std::size_t left = 0; left < outcomes.size(); ++left) {
    for (std::size_t right = left + 1; right < outcomes.size(); ++right) {
      const auto &a = outcomes[left];
      const auto &b = outcomes[right];
      relationship_effects.push_back(
          {.cat_a_id = a.cat_id,
           .cat_b_id = b.cat_id,
           .friendship_delta = success ? 1 : 0,
           .rivalry_delta = a.choice == b.choice ? 1 : 0,
           .respect_delta = a.mvp || b.mvp ? 1 : 0});
    }
  }

  return {.success = success,
          .team_score = team_score,
          .target_score = target_score,
          .fish_total = fish_total,
          .secret_found = secret_found,
          .strategy_bonus = strategy_bonus,
          .participants = std::move(outcomes),
          .relationship_effects = std::move(relationship_effects)};
}

} // namespace catforge::engine
