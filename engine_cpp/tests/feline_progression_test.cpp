#include "catforge/game_engine.hpp"
#include <algorithm>
#include <iostream>
#include <stdexcept>
using namespace catforge::engine;
void require(bool ok, const char *message) {
  if (!ok)
    throw std::runtime_error(message);
}
int main() {
  try {
    CatState starter{
        .id = 1, .breed = "british", .feline = BaseFelineStats("british")};
    auto first = Progress({.cat = starter,
                           .xp_gain = 100,
                           .seed = 1,
                           .source = "yard",
                           .outcome_tier = "fail"});
    require(first.cat.level == 2 && first.loot_item_id == "rat_tooth" &&
                first.cat.first_item_granted,
            "guaranteed first item");
    require(first.cat.feline.weight_grams > starter.feline.weight_grams,
            "level raises physical stats");
    auto arena = Progress({.cat = starter, .xp_gain = 100, .source = "arena"});
    auto arenaXP = Progress({.cat = first.cat,
                             .xp_gain = 999,
                             .source = "arena",
                             .outcome_tier = "win"});
    require(arenaXP.cat.xp == first.cat.xp + first.cat.level * 5,
            "arena XP is calculated by C++");
    require(arena.loot_item_id.empty() && !arena.cat.first_item_granted,
            "arena never drops power items");
    auto unlocked =
        Progress({.cat = first.cat, .xp_gain = 200, .source = "arena"});
    require(std::count(unlocked.facts.begin(), unlocked.facts.end(),
                       "arena_unlocked") == 1,
            "arena unlock occurs once");
    auto repeat = Progress({.cat = unlocked.cat, .source = "arena"});
    require(repeat.facts.empty() && repeat.cat.feline == unlocked.cat.feline,
            "no decay or repeat unlock");
    auto training = Train({.cat = starter,
                           .now_unix_nanos = 1000000000,
                           .random = {.training_crit_roll = 99, .encounter_roll = 50}});
    require(training.cat.level == 2 &&
                training.result.loot_item_id == "rat_tooth",
            "training first item");
    require(training.result.xp_gain == 113, "integer XP formula");
    auto rested =
        Train({.cat = training.cat,
               .now_unix_nanos = 1000000000 + 30LL * 24 * 60 * 60 * 1000000000,
               .random = {.training_crit_roll = 99, .encounter_roll = 50}});
    require(rested.cat.level >= training.cat.level &&
                rested.cat.feline.weight_grams >=
                    training.cat.feline.weight_grams,
            "offline never loses permanent progress");
    // Exhaust every possible independent training loot roll; loot should depend on energy,
    // without changing the existing balance or generating hidden currency.
    int drops[3]{};
    const int energy_sizes[]{50, 75, 100};
    for (int n = 0; n < 3; ++n) {
      for (unsigned seed = 0; seed < 10000; ++seed) {
        auto cat = starter;
        cat.level = 3;
        cat.coins = 777;
        cat.energy = energy_sizes[n];
        cat.first_item_granted = true;
        auto result = Train({.cat = cat, .now_unix_nanos = 1,
          .random = {.training_crit_roll = 99, .flavor_roll = static_cast<std::uint16_t>(seed), .training_loot_roll = static_cast<int>(seed)}});
        require(result.cat.coins == 777 && result.result.coins_gain == 0,
                "training preserves existing coins and awards none");
        require(result.result.energy_cost == energy_sizes[n], "loot uses actual spent energy");
        drops[n] += !result.result.loot_item_id.empty();
      }
      auto expected = energy_sizes[n] * 2;
      require(drops[n] == expected,
              "training loot rate differs from energy based expectation");
    }
    require(2 * drops[0] == drops[2],
            "splitting energy must not double expected loot");
    std::cout << "training drops per 10000 rolls (50/75/100): "
              << drops[0] << "/" << drops[1] << "/" << drops[2] << "\n";
    for (auto id : {"service_entry", "bird_knowledge", "dog_identity",
                    "foresight", "parkour"}) {
      auto type = std::string(id) == "service_entry" ||
                          std::string(id) == "bird_knowledge"
                      ? YardEventType::kFishTruck
                  : std::string(id) == "parkour" ? YardEventType::kBigBox
                                                 : YardEventType::kBigDog;
      auto choice =
          std::string(id) == "service_entry" || std::string(id) == "parkour"
              ? YardEventChoice::kSteal
          : std::string(id) == "dog_identity" ? YardEventChoice::kDistract
                                              : YardEventChoice::kScout;
      YardEventInput event{.seed = 42,
                           .event_type = type,
                           .participants = {{.cat_id = 1,
                                             .choice = choice,
                                             .feline = starter.feline,
                                             .special_action = id}}};
      bool blocked = false;
      try {
        ResolveYardEvent(event);
      } catch (const std::invalid_argument &) {
        blocked = true;
      }
      require(blocked, "missing item must block action");
      event.participants[0].effects = {id};
      auto allowed = ResolveYardEvent(event);
      require(allowed.participants.size() == 1, "equipped item opens action");
    }
    YardEventInput event{.seed = 42,
                         .event_type = YardEventType::kBigBox,
                         .participants = {{.cat_id = 1,
                                           .choice = YardEventChoice::kScout,
                                           .feline = starter.feline,
                                           .special_action = "ledge"}}};
    bool blocked = false;
    try {
      ResolveYardEvent(event);
    } catch (const std::invalid_argument &) {
      blocked = true;
    }
    require(blocked, "tail requirement enforced");
    event.participants[0].feline.tail_mm = 340;
    ResolveYardEvent(event);
    for (auto type : {YardEventType::kFishTruck, YardEventType::kBigDog,
                      YardEventType::kBigBox}) {
      YardEventInput low{.seed = 42,
                         .event_type = type,
                         .participants = {{.cat_id = 1,
                                           .choice = YardEventChoice::kSteal,
                                           .feline = starter.feline},
                                          {.cat_id = 2,
                                           .choice = YardEventChoice::kDistract,
                                           .feline = starter.feline}}};
      auto high = low;
      for (auto &c : high.participants) {
        c.level = 10;
        c.feline.weight_grams += 3000;
        c.feline.claws_tenth_mm += 100;
        c.feline.tail_mm += 200;
        c.feline.whisker_span_mm += 200;
      }
      auto l = ResolveYardEvent(low), h = ResolveYardEvent(high);
      require(l.target_score == h.target_score &&
                  h.outcome_tier > l.outcome_tier,
              "fixed event becomes easier");
      require(h.xp_gain > l.xp_gain, "tier controls participant XP");
    }
    // Full matrix uses independent deterministic seeds at progression
    // milestones.
    const std::string breeds[] = {"maine_coon", "siamese", "british", "bengal"};
    for (int level : {3, 5, 8, 10, 20})
      for (int a = 0; a < 4; a++)
        for (int b = a + 1; b < 4; b++) {
          CatState ca{.id = 1,
                      .breed = breeds[a],
                      .feline = BaseFelineStats(breeds[a])},
              cb{.id = 2,
                 .breed = breeds[b],
                 .feline = BaseFelineStats(breeds[b])};
          for (int i = 1; i < level; i++) {
            ca = Progress(
                     {.cat = ca, .xp_gain = ca.level * 100, .source = "arena"})
                     .cat;
            cb = Progress(
                     {.cat = cb, .xp_gain = cb.level * 100, .source = "arena"})
                     .cat;
          }
          int wins = 0;
          constexpr int seeds = 20000;
          for (int seed = 1; seed <= seeds; seed++) {
            auto f = Fight({.seed = static_cast<unsigned>(seed),
                            .cat_a = ca,
                            .cat_b = cb});
            wins += f.winner_cat_id == 1;
            require((f.final_hp_a == 0) != (f.final_hp_b == 0),
                    "combat ends with one winner");
          }
          require(wins >= seeds * 45 / 100 && wins <= seeds * 55 / 100,
                  "breed win rate outside 45-55%");
        }
    std::cout << "Feline progression and 600000 balance fights passed\n";
  } catch (const std::exception &e) {
    std::cerr << e.what() << '\n';
    return 1;
  }
}
