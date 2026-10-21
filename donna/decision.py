from __future__ import annotations

import math
from enum import StrEnum
from typing import Annotated

from pydantic import BaseModel, ConfigDict, Field, model_validator


Probability = Annotated[float, Field(strict=True, ge=0, le=1, allow_inf_nan=False)]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", strict=True)


class Action(StrEnum):
    ANSWER_DIRECTLY = "answer_directly"
    STRUCTURED_LOOKUP = "structured_lookup"
    SEMANTIC_SEARCH = "semantic_search"
    HYBRID_SEARCH = "hybrid_search"
    CLARIFY = "clarify"


class Scores(StrictModel):
    answer_directly: Probability
    structured_lookup: Probability
    semantic_search: Probability
    hybrid_search: Probability
    clarify: Probability

    @model_validator(mode="after")
    def validate_sum(self) -> Scores:
        if abs(sum(self.model_dump().values()) - 1) > 0.01:
            raise ValueError("probabilities must sum to 1")
        return self


class YesNoScores(StrictModel):
    yes: Probability
    no: Probability

    @model_validator(mode="after")
    def validate_sum(self) -> YesNoScores:
        if abs(self.yes + self.no - 1) > 0.01:
            raise ValueError("probabilities must sum to 1")
        return self


class DecisionResult(StrictModel):
    action: Action
    confidence: Probability


class CheckResult(StrictModel):
    answer: bool | None
    confidence: Probability


def _validate_minimum(minimum: float) -> None:
    if not math.isfinite(minimum) or not 0 <= minimum <= 1:
        raise ValueError("minimum confidence must be between 0 and 1")


def route(scores: Scores, minimum: float) -> DecisionResult:
    _validate_minimum(minimum)
    values = scores.model_dump()
    best = max(values.values())
    winners = [name for name, value in values.items() if value == best]
    if len(winners) != 1 or winners[0] == Action.CLARIFY or best < minimum:
        return DecisionResult(action=Action.CLARIFY, confidence=best)
    return DecisionResult(action=Action(winners[0]), confidence=best)


def evaluate(scores: YesNoScores, minimum: float) -> CheckResult:
    _validate_minimum(minimum)
    if scores.yes == scores.no:
        return CheckResult(answer=None, confidence=scores.yes)
    answer = scores.yes > scores.no
    confidence = max(scores.yes, scores.no)
    return CheckResult(answer=answer if confidence >= minimum else None, confidence=confidence)
