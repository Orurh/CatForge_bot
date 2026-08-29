#include "catforge/game_engine.hpp"

#include <algorithm>
#include <cstdlib>
#include <iostream>
#include <string_view>

namespace engine = catforge::engine;

namespace {

void Check(const bool condition, const std::string_view message) {
  if (!condition) {
    std::cerr << "FAILED: " << message << '\n';
    std::exit(1);
  }
}

void TestLevelUp() {
  constexpr std::int64_t now = 1'000'000'000'000;
  engine::TrainingInput input{
      .cat =
          {
              .breed = "bengal",
              .level = 1,
              .xp = 95,
              .energy = 100,
              .energy_updated_at_unix_nanos = now,
              .hp_base = 38,
              .atk_base = 26,
              .def_base = 16,
              .spd_base = 20,
          },
      .now_unix_nanos = now,
      .random = {.encounter_roll = 50, .flavor_roll = 7},
  };

  const auto output = engine::Train(input);
  Check(output.result.xp_gain == 135, "level-up XP gain");
  Check(output.result.energy_cost == 90, "level-up energy cost");
  Check(output.result.coins_gain == 9 && output.cat.coins == 9,
        "level-up coin reward");
  Check(output.result.encounter == engine::Encounter::kPigeon,
        "level-up encounter");
  Check(output.cat.level == 2 && output.cat.xp == 130 &&
            output.cat.energy == 10,
        "level-up state");
  Check(output.result.stats_gained ==
            engine::StatDelta{.hp = 3, .atk = 2, .def = 1, .spd = 2},
        "level-up stat delta");
}

void TestCriticalEfficiency() {
  constexpr std::int64_t now = 1'000'000'000'000;
  engine::TrainingInput input{
      .cat =
          {
              .level = 2,
              .energy = 100,
              .last_train_at_unix_nanos = now,
              .energy_updated_at_unix_nanos = now,
          },
      .now_unix_nanos = now,
      .random = {.encounter_roll = 0},
  };

  const auto output = engine::Train(input);
  Check(output.result.xp_gain == 136, "critical XP gain");
  Check(output.result.crit, "critical flag");
  Check(output.result.efficiency_percent == 50, "critical efficiency");
}

void TestTrainingUsesMostAvailableEnergy() {
  constexpr std::int64_t now = 1'000'000'000'000;
  struct Scenario {
    std::int32_t energy;
    std::int32_t expected_cost;
    std::int32_t expected_remaining;
    engine::TrainingOutcome expected_outcome;
  };
  constexpr Scenario scenarios[] = {
      {.energy = 100,
       .expected_cost = 90,
       .expected_remaining = 10,
       .expected_outcome = engine::TrainingOutcome::kOk},
      {.energy = 75,
       .expected_cost = 65,
       .expected_remaining = 10,
       .expected_outcome = engine::TrainingOutcome::kOk},
      {.energy = 40,
       .expected_cost = 30,
       .expected_remaining = 10,
       .expected_outcome = engine::TrainingOutcome::kOk},
      {.energy = 25,
       .expected_cost = 25,
       .expected_remaining = 0,
       .expected_outcome = engine::TrainingOutcome::kOk},
      {.energy = 24,
       .expected_cost = 0,
       .expected_remaining = 24,
       .expected_outcome = engine::TrainingOutcome::kNotEnoughEnergy},
  };
  for (const auto &scenario : scenarios) {
    const auto output = engine::Train({
        .cat = {.level = 1,
                .energy = scenario.energy,
                .energy_updated_at_unix_nanos = now},
        .now_unix_nanos = now,
        .random = {.encounter_roll = 50},
    });
    Check(output.result.outcome == scenario.expected_outcome,
          "training energy outcome");
    Check(output.result.energy_cost == scenario.expected_cost,
          "training dynamic energy cost");
    Check(output.cat.energy == scenario.expected_remaining,
          "training energy reserve");
    Check(output.cat.coins == (scenario.expected_cost > 0
                                   ? std::max(scenario.expected_cost / 10, 1)
                                   : 0),
          "training coin reward scales with cost");
  }
}

void TestInsufficientEnergy() {
  constexpr std::int64_t now = 1'000'000'000'000;
  engine::TrainingInput input{
      .cat =
          {
              .level = 1,
              .energy = 5,
              .energy_updated_at_unix_nanos =
                  now - 5 * engine::kEnergyRegenIntervalNanos,
          },
      .now_unix_nanos = now,
  };

  const auto output = engine::Train(input);
  Check(output.result.outcome == engine::TrainingOutcome::kNotEnoughEnergy,
        "insufficient-energy outcome");
  Check(output.cat.energy == 10, "insufficient-energy regeneration");
  Check(!output.cat.last_train_at_unix_nanos.has_value(),
        "insufficient-energy timestamp");
}

void TestExpeditionIsDeterministic() {
  constexpr std::int64_t now = 1'000'000'000'000;
  engine::ExpeditionInput input{
      .cat = {.breed = "bengal",
              .level = 1,
              .xp = 90,
              .energy = 100,
              .energy_updated_at_unix_nanos = now,
              .hp_base = 38,
              .atk_base = 26,
              .def_base = 16,
              .spd_base = 20},
      .now_unix_nanos = now,
      .seed = 42,
      .location = engine::ExpeditionLocation::kAlley,
      .difficulty = engine::ExpeditionDifficulty::kNormal,
      .loot_counts = {.common = 2, .rare = 2, .epic = 1},
      .force_loot = true,
  };

  const auto first = engine::Expedition(input);
  const auto second = engine::Expedition(input);
  Check(first.result.outcome == engine::ExpeditionOutcome::kVictory,
        "expedition victory");
  Check(first.result.turns == second.result.turns,
        "expedition deterministic turns");
  Check(first.result.enemy == second.result.enemy,
        "expedition deterministic enemy");
  Check(first.cat.energy == 70 && first.result.energy_cost == 30,
        "expedition energy cost");
  Check(first.result.xp_gain > 0 && first.result.coins_gain > 0 &&
            first.cat.coins == first.result.coins_gain &&
            !first.result.turns.empty(),
        "expedition reward and turns");
  Check(first.result.loot.dropped &&
            first.result.loot.rarity != engine::ItemRarity::kUnspecified,
        "forced expedition loot");
}

void TestEquipmentChangesCombatButNotBaseStats() {
  constexpr std::int64_t now = 1'000'000'000'000;
  const engine::ExpeditionInput base{
      .cat = {.breed = "bengal",
              .level = 1,
              .energy = 100,
              .energy_updated_at_unix_nanos = now,
              .hp_base = 40,
              .atk_base = 22,
              .def_base = 16,
              .spd_base = 20},
      .now_unix_nanos = now,
      .seed = 7,
      .location = engine::ExpeditionLocation::kAlley,
      .difficulty = engine::ExpeditionDifficulty::kEasy,
  };
  auto equipped = base;
  equipped.equipment_bonus = {.hp = 10, .atk = 8, .def = 5, .spd = 4};
  const auto plain_output = engine::Expedition(base);
  const auto equipped_output = engine::Expedition(equipped);
  Check(plain_output.result.turns != equipped_output.result.turns,
        "equipment changes combat");
  Check(equipped_output.cat.hp_base == base.cat.hp_base &&
            equipped_output.cat.atk_base == base.cat.atk_base,
        "equipment does not change base stats");
}

void TestExpeditionInsufficientEnergy() {
  constexpr std::int64_t now = 1'000'000'000'000;
  const auto output = engine::Expedition({
      .cat = {.level = 1,
              .energy = 3,
              .energy_updated_at_unix_nanos =
                  now - 5 * engine::kEnergyRegenIntervalNanos},
      .now_unix_nanos = now,
      .seed = 42,
      .location = engine::ExpeditionLocation::kPark,
      .difficulty = engine::ExpeditionDifficulty::kEasy,
  });
  Check(output.result.outcome == engine::ExpeditionOutcome::kNotEnoughEnergy,
        "expedition insufficient-energy outcome");
  Check(output.cat.energy == 8, "expedition insufficient-energy regeneration");
  Check(output.result.energy_cost == 0 && output.result.turns.empty(),
        "expedition insufficient-energy no battle");
}

void TestRareEnemiesExistForEveryLocation() {
  constexpr std::int64_t now = 1'000'000'000'000;
  std::uint64_t rare_seed = 0;
  bool found = false;
  for (std::uint64_t seed = 0; seed < 1000 && !found; ++seed) {
    const auto output = engine::Expedition({
        .cat = {.level = 5,
                .energy = 100,
                .energy_updated_at_unix_nanos = now,
                .hp_base = 500,
                .atk_base = 500,
                .def_base = 500,
                .spd_base = 500},
        .now_unix_nanos = now,
        .seed = seed,
        .location = engine::ExpeditionLocation::kAlley,
        .difficulty = engine::ExpeditionDifficulty::kNormal,
    });
    if (output.result.enemy.kind == engine::EnemyKind::kRatAccountant) {
      rare_seed = seed;
      found = true;
    }
  }
  Check(found, "rare alley enemy can be generated");

  const auto kind_at = [&](const engine::ExpeditionLocation location) {
    return engine::Expedition(
               {
                   .cat = {.level = 5,
                           .energy = 100,
                           .energy_updated_at_unix_nanos = now,
                           .hp_base = 500,
                           .atk_base = 500,
                           .def_base = 500,
                           .spd_base = 500},
                   .now_unix_nanos = now,
                   .seed = rare_seed,
                   .location = location,
                   .difficulty = engine::ExpeditionDifficulty::kNormal,
               })
        .result.enemy.kind;
  };
  Check(kind_at(engine::ExpeditionLocation::kRooftop) ==
            engine::EnemyKind::kCourierDog,
        "rare rooftop enemy kind");
  Check(kind_at(engine::ExpeditionLocation::kPark) ==
            engine::EnemyKind::kMoonLynx,
        "rare park enemy kind");
}

void TestYardEventIsDeterministicAndOrderIndependent() {
  const engine::YardEventInput input{
      .seed = 12345,
      .event_type = engine::YardEventType::kFishTruck,
      .participants = {{.cat_id = 20,
                        .choice = engine::YardEventChoice::kDistract,
                        .level = 2,
                        .hp = 40,
                        .atk = 18,
                        .def = 24,
                        .spd = 16},
                       {.cat_id = 10,
                        .choice = engine::YardEventChoice::kSteal,
                        .level = 2,
                        .hp = 35,
                        .atk = 28,
                        .def = 16,
                        .spd = 22},
                       {.cat_id = 30,
                        .choice = engine::YardEventChoice::kScout,
                        .level = 2,
                        .hp = 32,
                        .atk = 20,
                        .def = 15,
                        .spd = 30}}};
  auto reordered = input;
  std::reverse(reordered.participants.begin(), reordered.participants.end());

  const auto first = engine::ResolveYardEvent(input);
  const auto second = engine::ResolveYardEvent(reordered);
  Check(first == second, "yard event independent of input order");
  Check(first.success && first.strategy_bonus == 6,
        "yard event choice synergy");
  Check(first.fish_total > 0 && first.participants.size() == 3,
        "yard event rewards every participant");
  std::int32_t distributed = 0;
  std::int32_t mvp_count = 0;
  for (const auto &participant : first.participants) {
    distributed += participant.fish_reward;
    mvp_count += participant.mvp ? 1 : 0;
  }
  Check(distributed == first.fish_total && mvp_count == 1,
        "yard event reward accounting and MVP");
  Check(first.relationship_effects.size() == 3,
        "yard event emits every relationship pair");
  Check(first.relationship_effects[0].friendship_delta == 1 &&
            first.relationship_effects[0].respect_delta == 1,
        "yard event cooperation changes relationships");
}

void TestYardEventUsesRoleStat() {
  engine::YardEventInput input{
      .seed = 77,
      .event_type = engine::YardEventType::kFishTruck,
      .participants = {{.cat_id = 1,
                        .choice = engine::YardEventChoice::kScout,
                        .level = 1,
                        .hp = 20,
                        .atk = 20,
                        .def = 20,
                        .spd = 4}}};
  const auto slow = engine::ResolveYardEvent(input);
  input.participants[0].spd = 44;
  const auto fast = engine::ResolveYardEvent(input);
  Check(fast.participants[0].contribution - slow.participants[0].contribution ==
            10,
        "scout contribution uses speed");
}

void TestFightIsDeterministicAndProducesWinner() {
  const engine::FightInput input{
      .seed = 42,
      .cat_a = {.id = 10,
                .level = 5,
                .hp_base = 52,
                .atk_base = 30,
                .def_base = 22,
                .spd_base = 25},
      .cat_b = {.id = 20,
                .level = 5,
                .hp_base = 58,
                .atk_base = 27,
                .def_base = 25,
                .spd_base = 18},
  };
  const auto first = engine::Fight(input);
  const auto second = engine::Fight(input);
  Check(first == second, "fight is deterministic for a seed");
  Check((first.winner_cat_id == 10 || first.winner_cat_id == 20) &&
            first.loser_cat_id != first.winner_cat_id,
        "fight has one winner and one loser");
  Check(first.rounds > 0 && !first.turns.empty(), "fight produces turn log");
  Check(first.final_hp_a == 0 || first.final_hp_b == 0,
        "fight ends when a cat reaches zero HP");
}

} // namespace

int main() {
  TestLevelUp();
  TestCriticalEfficiency();
  TestInsufficientEnergy();
  TestTrainingUsesMostAvailableEnergy();
  TestExpeditionIsDeterministic();
  TestEquipmentChangesCombatButNotBaseStats();
  TestExpeditionInsufficientEnergy();
  TestRareEnemiesExistForEveryLocation();
  TestYardEventIsDeterministicAndOrderIndependent();
  TestYardEventUsesRoleStat();
  TestFightIsDeterministicAndProducesWinner();
  std::cout << "all C++ game-engine tests passed\n";
  return 0;
}
