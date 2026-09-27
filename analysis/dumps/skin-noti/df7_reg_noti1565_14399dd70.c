__int64 sub_14399DD70()
{
  __int64 v0; // rcx
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx
  int v3; // eax
  __int64 v4; // rax
  __int64 result; // rax
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
  v3 = sub_145693930(v0, 2403);
  v4 = sub_148AA307C(v3, 0, (unsigned int)&off_14DFF5FB0, (unsigned int)&off_14DDA74A0, 0);
  if ( v4 )
    *(_QWORD *)(v4 + 308) = v6;
  result = sub_14667EB40(qword_14E683C78, 1636);
  if ( result )
    return sub_14399DE40(result);
  return result;
}
