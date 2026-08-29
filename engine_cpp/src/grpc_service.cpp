#include "catforge/grpc_service.hpp"

#include "catforge/game_engine.hpp"

#include <exception>
#include <optional>
#include <utility>

namespace catforge::engine {
namespace pb = catforge::gameengine::v1;
namespace {

CatState FromProto(const pb::CatState &value) {
  CatState cat{
      .id = value.id(),
      .user_id = value.user_id(),
      .state_version = value.state_version(),
      .name = value.name(),
      .breed = value.breed(),
      .trait = value.trait(),
      .level = value.level(),
      .xp = value.xp(),
      .coins = value.coins(),
      .energy = value.energy(),
      .hp_base = value.hp_base(),
      .atk_base = value.atk_base(),
      .def_base = value.def_base(),
      .spd_base = value.spd_base(),
  };
  if (value.has_last_train_at_unix_nanos()) {
    cat.last_train_at_unix_nanos = value.last_train_at_unix_nanos();
  }
  if (value.has_energy_updated_at_unix_nanos()) {
    cat.energy_updated_at_unix_nanos = value.energy_updated_at_unix_nanos();
  }
  return cat;
}

void ToProto(const CatState &cat, pb::CatState *value) {
  value->set_id(cat.id);
  value->set_user_id(cat.user_id);
  value->set_state_version(cat.state_version);
  value->set_name(cat.name);
  value->set_breed(cat.breed);
  value->set_trait(cat.trait);
  value->set_level(cat.level);
  value->set_xp(cat.xp);
  value->set_coins(cat.coins);
  value->set_energy(cat.energy);
  if (cat.last_train_at_unix_nanos.has_value()) {
    value->set_last_train_at_unix_nanos(*cat.last_train_at_unix_nanos);
  }
  if (cat.energy_updated_at_unix_nanos.has_value()) {
    value->set_energy_updated_at_unix_nanos(*cat.energy_updated_at_unix_nanos);
  }
  value->set_hp_base(cat.hp_base);
  value->set_atk_base(cat.atk_base);
  value->set_def_base(cat.def_base);
  value->set_spd_base(cat.spd_base);
}

pb::Encounter ToProto(const Encounter encounter) {
  switch (encounter) {
  case Encounter::kPigeon:
    return pb::ENCOUNTER_PIGEON;
  case Encounter::kLizard:
    return pb::ENCOUNTER_LIZARD;
  case Encounter::kBigRat:
    return pb::ENCOUNTER_BIG_RAT;
  case Encounter::kMicePack:
    return pb::ENCOUNTER_MICE_PACK;
  case Encounter::kUnspecified:
  default:
    return pb::ENCOUNTER_UNSPECIFIED;
  }
}

void ToProto(const TrainResult &result, pb::TrainResult *value) {
  value->set_outcome(result.outcome == TrainingOutcome::kOk
                         ? pb::TRAINING_OUTCOME_OK
                         : pb::TRAINING_OUTCOME_NOT_ENOUGH_ENERGY);
  value->set_xp_gain(result.xp_gain);
  value->set_energy_cost(result.energy_cost);
  value->set_crit(result.crit);
  value->set_efficiency_percent(result.efficiency_percent);
  value->set_levels_gained(result.levels_gained);
  value->mutable_stats_gained()->set_hp(result.stats_gained.hp);
  value->mutable_stats_gained()->set_atk(result.stats_gained.atk);
  value->mutable_stats_gained()->set_def(result.stats_gained.def);
  value->mutable_stats_gained()->set_spd(result.stats_gained.spd);
  value->set_encounter(ToProto(result.encounter));
  value->set_flavor(result.flavor);
  value->set_coins_gain(result.coins_gain);
}

pb::EnemyKind ToProto(const EnemyKind kind) {
  switch (kind) {
  case EnemyKind::kSewerRat:
    return pb::ENEMY_KIND_SEWER_RAT;
  case EnemyKind::kStrayDog:
    return pb::ENEMY_KIND_STRAY_DOG;
  case EnemyKind::kWildLynx:
    return pb::ENEMY_KIND_WILD_LYNX;
  case EnemyKind::kRatAccountant:
    return pb::ENEMY_KIND_RAT_ACCOUNTANT;
  case EnemyKind::kCourierDog:
    return pb::ENEMY_KIND_COURIER_DOG;
  case EnemyKind::kMoonLynx:
    return pb::ENEMY_KIND_MOON_LYNX;
  default:
    return pb::ENEMY_KIND_UNSPECIFIED;
  }
}

ExpeditionLocation FromProto(const pb::ExpeditionLocation location) {
  switch (location) {
  case pb::EXPEDITION_LOCATION_ALLEY:
    return ExpeditionLocation::kAlley;
  case pb::EXPEDITION_LOCATION_ROOFTOP:
    return ExpeditionLocation::kRooftop;
  case pb::EXPEDITION_LOCATION_PARK:
    return ExpeditionLocation::kPark;
  default:
    return ExpeditionLocation::kUnspecified;
  }
}

ExpeditionDifficulty FromProto(const pb::ExpeditionDifficulty difficulty) {
  switch (difficulty) {
  case pb::EXPEDITION_DIFFICULTY_EASY:
    return ExpeditionDifficulty::kEasy;
  case pb::EXPEDITION_DIFFICULTY_NORMAL:
    return ExpeditionDifficulty::kNormal;
  case pb::EXPEDITION_DIFFICULTY_HARD:
    return ExpeditionDifficulty::kHard;
  default:
    return ExpeditionDifficulty::kUnspecified;
  }
}

pb::ExpeditionLocation ToProto(const ExpeditionLocation location) {
  switch (location) {
  case ExpeditionLocation::kAlley:
    return pb::EXPEDITION_LOCATION_ALLEY;
  case ExpeditionLocation::kRooftop:
    return pb::EXPEDITION_LOCATION_ROOFTOP;
  case ExpeditionLocation::kPark:
    return pb::EXPEDITION_LOCATION_PARK;
  default:
    return pb::EXPEDITION_LOCATION_UNSPECIFIED;
  }
}

pb::ExpeditionDifficulty ToProto(const ExpeditionDifficulty difficulty) {
  switch (difficulty) {
  case ExpeditionDifficulty::kEasy:
    return pb::EXPEDITION_DIFFICULTY_EASY;
  case ExpeditionDifficulty::kNormal:
    return pb::EXPEDITION_DIFFICULTY_NORMAL;
  case ExpeditionDifficulty::kHard:
    return pb::EXPEDITION_DIFFICULTY_HARD;
  default:
    return pb::EXPEDITION_DIFFICULTY_UNSPECIFIED;
  }
}

pb::ItemRarity ToProto(const ItemRarity rarity) {
  switch (rarity) {
  case ItemRarity::kCommon:
    return pb::ITEM_RARITY_COMMON;
  case ItemRarity::kRare:
    return pb::ITEM_RARITY_RARE;
  case ItemRarity::kEpic:
    return pb::ITEM_RARITY_EPIC;
  default:
    return pb::ITEM_RARITY_UNSPECIFIED;
  }
}

void ToProto(const ExpeditionResult &result, pb::ExpeditionResult *value) {
  switch (result.outcome) {
  case ExpeditionOutcome::kVictory:
    value->set_outcome(pb::EXPEDITION_OUTCOME_VICTORY);
    break;
  case ExpeditionOutcome::kDefeat:
    value->set_outcome(pb::EXPEDITION_OUTCOME_DEFEAT);
    break;
  case ExpeditionOutcome::kNotEnoughEnergy:
    value->set_outcome(pb::EXPEDITION_OUTCOME_NOT_ENOUGH_ENERGY);
    break;
  }
  auto *enemy = value->mutable_enemy();
  enemy->set_kind(ToProto(result.enemy.kind));
  enemy->set_level(result.enemy.level);
  enemy->set_hp(result.enemy.hp);
  enemy->set_atk(result.enemy.atk);
  enemy->set_def(result.enemy.def);
  enemy->set_spd(result.enemy.spd);
  value->set_energy_cost(result.energy_cost);
  value->set_xp_gain(result.xp_gain);
  value->set_coins_gain(result.coins_gain);
  value->mutable_loot()->set_dropped(result.loot.dropped);
  value->mutable_loot()->set_rarity(ToProto(result.loot.rarity));
  value->mutable_loot()->set_item_index(result.loot.item_index);
  value->set_location(ToProto(result.location));
  value->set_difficulty(ToProto(result.difficulty));
  value->set_rounds(result.rounds);
  value->set_cat_hp_after(result.cat_hp_after);
  value->set_enemy_hp_after(result.enemy_hp_after);
  value->set_levels_gained(result.levels_gained);
  value->mutable_stats_gained()->set_hp(result.stats_gained.hp);
  value->mutable_stats_gained()->set_atk(result.stats_gained.atk);
  value->mutable_stats_gained()->set_def(result.stats_gained.def);
  value->mutable_stats_gained()->set_spd(result.stats_gained.spd);
  for (const auto &turn : result.turns) {
    auto *target = value->add_turns();
    target->set_round(turn.round);
    target->set_actor(turn.actor == BattleActor::kCat ? pb::BATTLE_ACTOR_CAT
                                                      : pb::BATTLE_ACTOR_ENEMY);
    target->set_damage(turn.damage);
    target->set_crit(turn.crit);
    target->set_defender_hp_after(turn.defender_hp_after);
  }
}

YardEventType FromProto(const pb::YardEventType value) {
  switch (value) {
  case pb::YARD_EVENT_TYPE_FISH_TRUCK:
    return YardEventType::kFishTruck;
  default:
    return YardEventType::kUnspecified;
  }
}

YardEventChoice FromProto(const pb::YardEventChoice value) {
  switch (value) {
  case pb::YARD_EVENT_CHOICE_STEAL:
    return YardEventChoice::kSteal;
  case pb::YARD_EVENT_CHOICE_DISTRACT:
    return YardEventChoice::kDistract;
  case pb::YARD_EVENT_CHOICE_SCOUT:
    return YardEventChoice::kScout;
  default:
    return YardEventChoice::kUnspecified;
  }
}

pb::YardEventChoice ToProto(const YardEventChoice value) {
  switch (value) {
  case YardEventChoice::kSteal:
    return pb::YARD_EVENT_CHOICE_STEAL;
  case YardEventChoice::kDistract:
    return pb::YARD_EVENT_CHOICE_DISTRACT;
  case YardEventChoice::kScout:
    return pb::YARD_EVENT_CHOICE_SCOUT;
  default:
    return pb::YARD_EVENT_CHOICE_UNSPECIFIED;
  }
}

void ToProto(const YardEventResult &result,
             pb::ResolveYardEventResponse *value) {
  value->set_success(result.success);
  value->set_team_score(result.team_score);
  value->set_target_score(result.target_score);
  value->set_fish_total(result.fish_total);
  value->set_secret_found(result.secret_found);
  value->set_strategy_bonus(result.strategy_bonus);
  for (const auto &participant : result.participants) {
    auto *target = value->add_participants();
    target->set_cat_id(participant.cat_id);
    target->set_choice(ToProto(participant.choice));
    target->set_contribution(participant.contribution);
    target->set_fish_reward(participant.fish_reward);
    target->set_mvp(participant.mvp);
  }
  for (const auto &effect : result.relationship_effects) {
    auto *target = value->add_relationship_effects();
    target->set_cat_a_id(effect.cat_a_id);
    target->set_cat_b_id(effect.cat_b_id);
    target->set_friendship_delta(effect.friendship_delta);
    target->set_rivalry_delta(effect.rivalry_delta);
    target->set_respect_delta(effect.respect_delta);
  }
}

void ToProto(const FightResult &result, pb::FightResponse *value) {
  value->set_winner_cat_id(result.winner_cat_id);
  value->set_loser_cat_id(result.loser_cat_id);
  value->set_rounds(result.rounds);
  value->set_final_hp_a(result.final_hp_a);
  value->set_final_hp_b(result.final_hp_b);
  for (const auto &turn : result.turns) {
    auto *target = value->add_turns();
    target->set_round(turn.round);
    target->set_attacker_cat_id(turn.attacker_cat_id);
    target->set_defender_cat_id(turn.defender_cat_id);
    target->set_damage(turn.damage);
    target->set_crit(turn.crit);
    target->set_defender_hp_after(turn.defender_hp_after);
  }
}

} // namespace

grpc::Status GameEngineService::Train(grpc::ServerContext *,
                                      const pb::TrainRequest *request,
                                      pb::TrainResponse *response) {
  if (!request->has_cat() || !request->has_random()) {
    return {grpc::StatusCode::INVALID_ARGUMENT, "cat and random are required"};
  }

  try {
    const auto output = engine::Train({
        .rules_version = request->rules_version(),
        .content_version = request->content_version(),
        .cat = FromProto(request->cat()),
        .now_unix_nanos = request->now_unix_nanos(),
        .random =
            {
                .energy_cost_roll = request->random().energy_cost_roll(),
                .xp_gain_roll = request->random().xp_gain_roll(),
                .encounter_roll = request->random().encounter_roll(),
                .flavor_roll = request->random().flavor_roll(),
            },
    });
    ToProto(output.cat, response->mutable_cat());
    ToProto(output.result, response->mutable_result());
    return grpc::Status::OK;
  } catch (const std::invalid_argument &error) {
    return {grpc::StatusCode::INVALID_ARGUMENT, error.what()};
  } catch (const std::exception &error) {
    return {grpc::StatusCode::INTERNAL, error.what()};
  }
}

grpc::Status GameEngineService::Expedition(grpc::ServerContext *,
                                           const pb::ExpeditionRequest *request,
                                           pb::ExpeditionResponse *response) {
  if (!request->has_cat()) {
    return {grpc::StatusCode::INVALID_ARGUMENT, "cat is required"};
  }

  try {
    const auto output = engine::Expedition({
        .rules_version = request->rules_version(),
        .content_version = request->content_version(),
        .cat = FromProto(request->cat()),
        .now_unix_nanos = request->now_unix_nanos(),
        .seed = request->seed(),
        .location = FromProto(request->location()),
        .difficulty = FromProto(request->difficulty()),
        .equipment_bonus = {.hp = request->equipment_bonus().hp(),
                            .atk = request->equipment_bonus().atk(),
                            .def = request->equipment_bonus().def(),
                            .spd = request->equipment_bonus().spd()},
        .loot_counts = {.common = request->loot_counts().common(),
                        .rare = request->loot_counts().rare(),
                        .epic = request->loot_counts().epic()},
        .force_loot = request->force_loot(),
    });
    ToProto(output.cat, response->mutable_cat());
    ToProto(output.result, response->mutable_result());
    return grpc::Status::OK;
  } catch (const std::invalid_argument &error) {
    return {grpc::StatusCode::INVALID_ARGUMENT, error.what()};
  } catch (const std::exception &error) {
    return {grpc::StatusCode::INTERNAL, error.what()};
  }
}

grpc::Status
GameEngineService::ResolveYardEvent(grpc::ServerContext *,
                                    const pb::ResolveYardEventRequest *request,
                                    pb::ResolveYardEventResponse *response) {
  try {
    std::vector<YardEventParticipant> participants;
    participants.reserve(
        static_cast<std::size_t>(request->participants_size()));
    for (const auto &participant : request->participants()) {
      participants.push_back({.cat_id = participant.cat_id(),
                              .choice = FromProto(participant.choice()),
                              .level = participant.level(),
                              .hp = participant.hp(),
                              .atk = participant.atk(),
                              .def = participant.def(),
                              .spd = participant.spd()});
    }
    const auto result = engine::ResolveYardEvent(
        {.rules_version = request->rules_version(),
         .content_version = request->content_version(),
         .seed = request->seed(),
         .event_type = FromProto(request->event_type()),
         .participants = std::move(participants)});
    ToProto(result, response);
    return grpc::Status::OK;
  } catch (const std::invalid_argument &error) {
    return {grpc::StatusCode::INVALID_ARGUMENT, error.what()};
  } catch (const std::exception &error) {
    return {grpc::StatusCode::INTERNAL, error.what()};
  }
}

grpc::Status GameEngineService::Fight(grpc::ServerContext *,
                                      const pb::FightRequest *request,
                                      pb::FightResponse *response) {
  if (!request->has_cat_a() || !request->has_cat_b()) {
    return {grpc::StatusCode::INVALID_ARGUMENT, "two cats are required"};
  }
  try {
    const auto result =
        engine::Fight({.rules_version = request->rules_version(),
                       .content_version = request->content_version(),
                       .seed = request->seed(),
                       .cat_a = FromProto(request->cat_a()),
                       .cat_b = FromProto(request->cat_b())});
    ToProto(result, response);
    return grpc::Status::OK;
  } catch (const std::invalid_argument &error) {
    return {grpc::StatusCode::INVALID_ARGUMENT, error.what()};
  } catch (const std::exception &error) {
    return {grpc::StatusCode::INTERNAL, error.what()};
  }
}

} // namespace catforge::engine
