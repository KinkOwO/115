#include "preparation-state.h"
#include <stdio.h>
#include <string.h>
int main() {
    ElitePreparationState s = {};
    if (ElitePreparationMatches(s, 1, 121, 7, 1)) return 1;
    EliteArmPreparation(s, 1, 121, 7, 50);
    if (!ElitePreparationMatches(s, 1, 121, 7, 51) ||
        ElitePreparationMatches(s, 2, 121, 7, 51) ||
        ElitePreparationMatches(s, 1, 120, 7, 51) ||
        ElitePreparationMatches(s, 1, 121, 8, 51) ||
        ElitePreparationMatches(s, 1, 121, 7, 10050)) return 2;
    if (!ElitePreparationCaller(1754, 0x2E5A5A2) || !ElitePreparationCaller(1382, 0x44FD1CF) ||
        !ElitePreparationCaller(1879, 0x44FAC6B) || ElitePreparationCaller(1382, 0x5B251C3) ||
        ElitePreparationCaller(1879, 0x44FD019) || ElitePreparationCaller(312, 0x44FD019)) return 3;
    unsigned char body[1+531*2] = {};
    body[0] = 2; body[14] = 1; body[1+531+13] = 2;
    memset(body+1+531+39, 255, 12);
    bool nonEmpty = true;
    if (!ElitePreparationSelection(body, sizeof(body), nonEmpty) || nonEmpty) return 4;
    body[1+531+39] = 0; body[1+531+40] = 0; body[1+531+41] = 0; body[1+531+42] = 0;
    if (!ElitePreparationSelection(body, sizeof(body), nonEmpty) || !nonEmpty) return 5;
    if (ElitePreparationSelection(body, sizeof(body)-1, nonEmpty)) return 6;
    body[14] = 2;
    if (ElitePreparationSelection(body, sizeof(body), nonEmpty)) return 7;
    body[0] = 0;
    if (ElitePreparationSelection(body, sizeof(body), nonEmpty)) return 8;
    puts("preparation state: native row layouts, empty/duplicate/truncated rejection, caller and transaction boundaries passed");
    return 0;
}
