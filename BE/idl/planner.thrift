namespace go planner
include "model.thrift"
service PlannerService {
  model.PlanningResult GenerateItinerary(1: model.PlanningRequest req)
  model.PlanningResult ReviseItinerary(1: model.PlanningRequest req)
}
