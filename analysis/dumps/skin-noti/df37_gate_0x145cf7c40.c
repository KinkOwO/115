// gate_0x145cf7c40

bool __fastcall sub_145CF7C40(__int64 a1, int a2)
{
  int v4; // esi
  bool result; // al

  result = (unsigned int)sub_145CD39F0(a1, 2) == a2
        || (v4 = *(_DWORD *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 7656LL))(a1) + 12),
            (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4832LL))(a1) == 4)
        && v4 == 1
        && !(*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)a1 + 10096LL))(a1)
        && a2 == 300
        || (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4832LL))(a1) == 10 && a2 == 261;
  return result;
}

