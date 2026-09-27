// sub_1444EC530

bool __fastcall sub_1444EC530(__int64 a1, __int64 a2, __int64 a3)
{
  __int64 v3; // rax

  LOBYTE(a3) = 1;
  v3 = sub_140283D60(qword_14E683BF8, a2, a3);
  return v3 && *(_DWORD *)(v3 + 8) == 1 && *(_DWORD *)(v3 + 12) == 3;
}

