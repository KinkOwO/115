// nb_sub_146ED00C0

bool __fastcall sub_146ED00C0(__int64 a1)
{
  __int64 v2; // rax
  float v3; // xmm0_4
  float v4; // xmm0_4
  bool result; // al

  v2 = sub_146EC45D0(*(_QWORD *)(a1 + 400));
  v3 = *(float *)(a1 + 168);
  result = 0;
  if ( (float)(int)v2 >= v3 && (float)(int)v2 <= (float)(v3 + *(float *)(a1 + 152)) )
  {
    v4 = *(float *)(a1 + 172);
    if ( (float)SHIDWORD(v2) >= v4 && (float)SHIDWORD(v2) <= (float)(v4 + *(float *)(a1 + 156)) )
      return 1;
  }
  return result;
}

