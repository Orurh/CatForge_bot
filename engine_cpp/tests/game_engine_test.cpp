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
      .random = {.training_crit_roll = 99, .xp_gain_roll = 25, .encounter_roll = 50, .flavor_roll = 7},
  };

  const auto output = engine::Train(input);
  Check(output.result.xp_gain == 150, "level-up XP gain");
  Check(output.result.energy_cost == 100, "level-up energy cost");
  Check(output.result.coins_gain == 0 && output.cat.coins == 0,
        "training never mints legacy coins");
  Check(output.result.encounter == engine::Encounter::kPigeon,
        "level-up encounter");
  Check(output.cat.level == 2 && output.cat.xp == 145 &&
            output.cat.energy == 0,
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
      .random = {.training_crit_roll = 0, .xp_gain_roll = 25, .encounter_roll = 0},
  };

  const auto output = engine::Train(input);
  Check(output.result.xp_gain == 300, "critical XP gain");
  Check(output.result.crit, "critical flag");
  Check(output.result.efficiency_percent == 100, "critical efficiency");
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
      {100,100,0,engine::TrainingOutcome::kOk},
      {75,75,0,engine::TrainingOutcome::kOk},
      {50,50,0,engine::TrainingOutcome::kOk},
      {49,0,49,engine::TrainingOutcome::kNotEnoughEnergy},
      {0,0,0,engine::TrainingOutcome::kNotEnoughEnergy},
  };
  for (const auto &scenario : scenarios) {
    const auto output = engine::Train({
        .cat = {.level = 1,
                .energy = scenario.energy,
                .energy_updated_at_unix_nanos = now},
        .now_unix_nanos = now,
        .random = {.training_crit_roll = 99, .encounter_roll = 50},
    });
    Check(output.result.outcome == scenario.expected_outcome,
          "training energy outcome");
    Check(output.result.energy_cost == scenario.expected_cost,
          "training dynamic energy cost");
    Check(output.cat.energy == scenario.expected_remaining,
          "training energy reserve");
    Check(output.cat.coins == 0,
          "training has no hidden coins");
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
                        .level = 8,
                        .hp = 40,
                        .atk = 18,
                        .def = 24,
                        .spd = 16},
                       {.cat_id = 10,
                        .choice = engine::YardEventChoice::kSteal,
                        .level = 8,
                        .hp = 35,
                        .atk = 28,
                        .def = 16,
                        .spd = 22},
                       {.cat_id = 30,
                        .choice = engine::YardEventChoice::kScout,
                        .level = 8,
                        .hp = 32,
                        .atk = 20,
                        .def = 15,
                        .spd = 30}}};
  auto reordered = input;
  std::reverse(reordered.participants.begin(), reordered.participants.end());

  const auto first = engine::ResolveYardEvent(input);
  const auto second = engine::ResolveYardEvent(reordered);
  Check(first == second, "yard event independent of input order");
  Check((first.outcome_tier == engine::YardEventOutcomeTier::kSuccess ||
         first.outcome_tier == engine::YardEventOutcomeTier::kExceptional) &&
            first.strategy_bonus == 6,
        "yard event choice synergy");
  Check(first.yard_score > 0 && first.xp_gain >= 30 &&
            first.participants.size() == 3,
        "yard event rewards every participant");
  std::int32_t mvp_count = 0;
  for (const auto &participant : first.participants) {
    mvp_count += participant.mvp ? 1 : 0;
  }
  Check(mvp_count == 1, "yard event keeps one MVP without personal loot");
  Check(first.relationship_effects.size() == 3,
        "yard event emits every relationship pair");
  std::int32_t friendship_edges = 0;
  std::int32_t respect_edges = 0;
  for (const auto &effect : first.relationship_effects) {
    friendship_edges += effect.friendship_delta;
    respect_edges += effect.respect_delta;
  }
  Check(friendship_edges == 3 && respect_edges == 2,
        "yard event cooperation and MVP change relationships");
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
            5,
        "scout contribution uses speed");
}

void TestAllYardEventTemplatesResolve() {
  for (const auto event_type :
       {engine::YardEventType::kFishTruck, engine::YardEventType::kBigDog,
        engine::YardEventType::kBigBox}) {
    const engine::YardEventInput input{
        .seed = 991,
        .event_type = event_type,
        .participants = {{.cat_id = 1,
                          .choice = engine::YardEventChoice::kSteal,
                          .level = 3,
                          .hp = 40,
                          .atk = 28,
                          .def = 20,
                          .spd = 24},
                         {.cat_id = 2,
                          .choice = engine::YardEventChoice::kDistract,
                          .level = 3,
                          .hp = 48,
                          .atk = 20,
                          .def = 27,
                          .spd = 19},
                         {.cat_id = 3,
                          .choice = engine::YardEventChoice::kScout,
                          .level = 3,
                          .hp = 35,
                          .atk = 23,
                          .def = 18,
                          .spd = 31}}};
    const auto result = engine::ResolveYardEvent(input);
    Check(result.participants.size() == input.participants.size(),
          "every yard event template resolves all participants");
    Check(result.yard_score > 0,
          "every yard event template produces event points");
  }
}

void TestLegacyFishTruckCanFinishAfterContentUpgrade() {
  const engine::YardEventInput input{
      .content_version = 2,
      .seed = 41,
      .event_type = engine::YardEventType::kFishTruck,
      .participants = {{.cat_id = 1,
                        .choice = engine::YardEventChoice::kSteal,
                        .level = 2,
                        .hp = 32,
                        .atk = 24,
                        .def = 18,
                        .spd = 21}}};
  const auto result = engine::ResolveYardEvent(input);
  Check(result.participants.size() == 1,
        "content v2 fish truck remains resolvable after upgrade");
  Check(result.outcome_tier == engine::YardEventOutcomeTier::kFailure ||
            result.outcome_tier == engine::YardEventOutcomeTier::kPartial,
        "solo event cannot produce a full success");
}

void TestYardEventTargetRemainsFixed() {
  engine::YardEventInput low{
      .seed = 17,
      .event_type = engine::YardEventType::kFishTruck,
      .participants = {{.cat_id = 1,
                        .choice = engine::YardEventChoice::kSteal,
                        .level = 1,
                        .hp = 20,
                        .atk = 20,
                        .def = 20,
                        .spd = 20},
                       {.cat_id = 2,
                        .choice = engine::YardEventChoice::kDistract,
                        .level = 1,
                        .hp = 20,
                        .atk = 20,
                        .def = 20,
                        .spd = 20}}};
  auto high = low;
  for (auto &cat : high.participants) {
    cat.level = 15;
    cat.hp = cat.atk = cat.def = cat.spd = 60;
  }
  const auto low_result = engine::ResolveYardEvent(low);
  const auto high_result = engine::ResolveYardEvent(high);
  Check(high_result.target_score == low_result.target_score && high_result.outcome_tier > low_result.outcome_tier,
        "training improves results against a fixed target");
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

double FightWinRate(const engine::CatState &cat_a,
                    const engine::CatState &cat_b,
                    const std::int32_t battles = 3000) {
  std::int32_t wins = 0;
  for (std::int32_t seed = 1; seed <= battles; ++seed) {
    const auto result = engine::Fight({.seed = static_cast<std::uint64_t>(seed),
                                       .cat_a = cat_a,
                                       .cat_b = cat_b});
    if (result.winner_cat_id == cat_a.id) {
      ++wins;
    }
  }
  return static_cast<double>(wins) / battles;
}

void TestFightBalanceBands() {
  const std::vector<engine::CatState> level_5{
      {.id = 10,
       .breed = "maine_coon",
       .level = 5,
       .hp_base = 62,
       .atk_base = 28,
       .def_base = 26,
       .spd_base = 18},
      {.id = 20,
       .breed = "siamese",
       .level = 5,
       .hp_base = 58,
       .atk_base = 26,
       .def_base = 22,
       .spd_base = 34},
      {.id = 30,
       .breed = "british",
       .level = 5,
       .hp_base = 54,
       .atk_base = 28,
       .def_base = 30,
       .spd_base = 20},
      {.id = 40,
       .breed = "bengal",
       .level = 5,
       .hp_base = 52,
       .atk_base = 30,
       .def_base = 20,
       .spd_base = 28},
  };
  const std::vector<engine::CatState> level_8{
      {.id = 11,
       .breed = "maine_coon",
       .level = 8,
       .hp_base = 74,
       .atk_base = 34,
       .def_base = 32,
       .spd_base = 21},
      {.id = 21,
       .breed = "siamese",
       .level = 8,
       .hp_base = 70,
       .atk_base = 32,
       .def_base = 25,
       .spd_base = 40},
      {.id = 31,
       .breed = "british",
       .level = 8,
       .hp_base = 63,
       .atk_base = 34,
       .def_base = 36,
       .spd_base = 23},
      {.id = 41,
       .breed = "bengal",
       .level = 8,
       .hp_base = 61,
       .atk_base = 36,
       .def_base = 23,
       .spd_base = 34},
  };
  const std::vector<engine::CatState> level_10{
      {.id = 12,
       .breed = "maine_coon",
       .level = 10,
       .hp_base = 82,
       .atk_base = 38,
       .def_base = 36,
       .spd_base = 23},
      {.id = 22,
       .breed = "siamese",
       .level = 10,
       .hp_base = 78,
       .atk_base = 36,
       .def_base = 27,
       .spd_base = 44},
      {.id = 32,
       .breed = "british",
       .level = 10,
       .hp_base = 69,
       .atk_base = 38,
       .def_base = 40,
       .spd_base = 25},
      {.id = 42,
       .breed = "bengal",
       .level = 10,
       .hp_base = 67,
       .atk_base = 40,
       .def_base = 25,
       .spd_base = 38},
  };

  for (const auto &cat_a : level_5) {
    for (const auto &cat_b : level_5) {
      if (cat_a.id == cat_b.id) {
        continue;
      }
      const auto rate = FightWinRate(cat_a, cat_b);
      Check(rate >= 0.43 && rate <= 0.57,
            "equal-level breed matchup stays competitive");
    }
  }
  for (const auto &cat_a : level_8) {
    for (const auto &cat_b : level_5) {
      const auto rate = FightWinRate(cat_a, cat_b);
      Check(rate >= 0.60 && rate <= 0.75,
            "three-level advantage stays meaningful but uncertain");
    }
  }
  for (const auto &cat_a : level_10) {
    for (const auto &cat_b : level_5) {
      const auto rate = FightWinRate(cat_a, cat_b);
      Check(rate >= 0.68 && rate <= 0.85,
            "five-level advantage never becomes deterministic");
    }
  }
}


void TestTrainingV2MathAndRegen() {
  constexpr std::int64_t now = 10'000'000'000'000;
  engine::TrainingInput input{.cat={.level=1,.energy=100,.energy_updated_at_unix_nanos=now},.now_unix_nanos=now};
  for (const auto energy : {50,100}) {
    input.cat.energy=energy;
    for (const auto roll : {0,50}) {
      input.random.xp_gain_roll=roll;
      input.random.training_crit_roll=99;
      auto normal=engine::Train(input);
      const auto expected=energy==100 ? (roll==0 ? 113 : 188) : (roll==0 ? 56 : 94);
      Check(normal.result.xp_gain==expected,"v2 normal XP bounds");
      input.random.training_crit_roll=0;
      auto critical=engine::Train(input);
      Check(critical.result.xp_gain==expected+energy*3/2,"crit adds base rather than multiplying random XP");
      input.cat.level=20; input.cat.last_train_at_unix_nanos=now;
      Check(engine::Train(input).result.xp_gain==critical.result.xp_gain,"no level or frequency XP multiplier");
      input.cat.level=1;
    }
  }
  input.cat.energy=100;
  for (const auto bonus : {0,2,3,99}) {
    input.training_crit_bonus_percent=bonus;
    int crits=0;
    for (int roll=0;roll<100;++roll) {
      input.random.training_crit_roll=roll;
      crits+=engine::Train(input).result.crit ? 1 : 0;
    }
    Check(crits==5+std::min(bonus,5),"5/7/8 percent crit and hard cap 10");
  }
  input.training_crit_bonus_percent=0;
  input.cat.energy=49;
  input.cat.energy_updated_at_unix_nanos=now;
  input.now_unix_nanos=now+7*60'000'000'000;
  auto refused=engine::Train(input);
  Check(refused.result.outcome==engine::TrainingOutcome::kNotEnoughEnergy,"49 cannot train");
  Check(refused.cat.energy_updated_at_unix_nanos==now,"refusal preserves fractional regen");
  input.cat=refused.cat; input.now_unix_nanos=now+engine::kEnergyRegenIntervalNanos;
  auto allowed=engine::Train(input);
  Check(allowed.result.energy_cost==50 && allowed.cat.energy==0,"retry at 8 minutes succeeds and spends all");
  input.cat.energy=50; input.cat.energy_updated_at_unix_nanos=now;
  input.now_unix_nanos=now+7*60'000'000'000;
  auto partial=engine::Train(input);
  Check(partial.cat.energy_updated_at_unix_nanos==now,"successful training preserves fractional regen too");
}

} // namespace

int main() {
  TestTrainingV2MathAndRegen();
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
  TestAllYardEventTemplatesResolve();
  TestLegacyFishTruckCanFinishAfterContentUpgrade();
  TestYardEventTargetRemainsFixed();
  TestFightIsDeterministicAndProducesWinner();
  TestFightBalanceBands();
  std::cout << "all C++ game-engine tests passed\n";
  return 0;
}
