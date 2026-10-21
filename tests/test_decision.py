import math

import pytest
from pydantic import ValidationError

from donna.decision import Action, Scores, YesNoScores, evaluate, route


def test_route_returns_unique_highest_action():
    scores = Scores(answer_directly=0.1, structured_lookup=0.1, semantic_search=0.6, hybrid_search=0.1, clarify=0.1)
    assert route(scores, 0.55).action is Action.SEMANTIC_SEARCH


@pytest.mark.parametrize("values", [
    {"answer_directly": 0.2, "structured_lookup": 0.2, "semantic_search": 0.2, "hybrid_search": 0.2, "clarify": 0.2, "extra": 0},
    {"answer_directly": 0.5, "structured_lookup": 0.1, "semantic_search": 0.1, "hybrid_search": 0.1, "clarify": 0.1},
    {"answer_directly": float("nan"), "structured_lookup": 0.1, "semantic_search": 0.1, "hybrid_search": 0.1, "clarify": 0.1},
])
def test_scores_reject_extra_non_normalized_and_non_finite(values):
    with pytest.raises(ValidationError):
        Scores.model_validate(values)


def test_route_tie_and_low_confidence_clarify():
    tied = Scores(answer_directly=0.4, structured_lookup=0.4, semantic_search=0.05, hybrid_search=0.05, clarify=0.1)
    low = Scores(answer_directly=0.5, structured_lookup=0.1, semantic_search=0.1, hybrid_search=0.1, clarify=0.2)
    assert route(tied, 0.55).action is Action.CLARIFY
    assert route(low, 0.6).action is Action.CLARIFY


def test_yes_no_tie_remains_unknown_and_definite_clears_threshold():
    tie = YesNoScores(yes=0.5, no=0.5)
    yes = YesNoScores(yes=0.8, no=0.2)
    assert evaluate(tie, 0.55).answer is None
    assert evaluate(yes, 0.55).answer is True


def test_invalid_minimum_is_rejected():
    scores = YesNoScores(yes=0.5, no=0.5)
    with pytest.raises(ValueError):
        evaluate(scores, math.nan)
