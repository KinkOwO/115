__int64 sub_1422A4F70()
{
  __int64 v0; // rcx
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx
  int v3; // eax
  __int64 result; // rax
  __int64 v5; // rbx
  __int64 v6; // [rsp+60h] [rbp+18h] BYREF
  __int64 v7; // [rsp+68h] [rbp+20h]

  v6 = 0;
  sub_146EA0BE0(&v6, 8);
  v0 = qword_14E634248;
  if ( !qword_14E634248 )
  {
    v1 = sub_146E8BA20(2496);
    v7 = v1;
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_1456918F0(v1);
    else
      v2 = 0;
    qword_14E634248 = (__int64)v2;
    (**v2)(v2);
    v0 = qword_14E634248;
  }
  v3 = sub_145693930(v0, 620);
  result = sub_148AA307C(v3, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DD969A8, 0);
  v5 = result;
  if ( result )
  {
    *(_QWORD *)(result + 160) = v6;
    sub_146E9FBC0(result + 168);
    result = (unsigned int)sub_140B84E90(v5 + 72) - *(_DWORD *)(v5 + 160);
    if ( (int)result > 0 )
      return sub_146E9FF10(v5 + 168, (unsigned int)(1000 * result), 0);
  }
  return result;
}
