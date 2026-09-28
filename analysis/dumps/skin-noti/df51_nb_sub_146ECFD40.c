// nb_sub_146ECFD40

__int64 __fastcall sub_146ECFD40(__int64 a1)
{
  _QWORD *v1; // rbx
  __int64 result; // rax

  v1 = (_QWORD *)(a1 + 288);
  sub_1417BACB0(a1 + 288, *(_QWORD *)(a1 + 288));
  *(_QWORD *)*v1 = *v1;
  result = *v1;
  *(_QWORD *)(*v1 + 8LL) = *v1;
  v1[1] = 0;
  return result;
}

