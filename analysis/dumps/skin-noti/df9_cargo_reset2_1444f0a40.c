__int64 __fastcall sub_1444F0A40(__int64 a1)
{
  unsigned int v2; // ebx
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx

  v2 = 0;
  v3 = qword_14E64EBF0;
  if ( !qword_14E64EBF0 )
  {
    v4 = sub_146E8BA20(104);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_145633BF0(v4);
    else
      v5 = 0;
    qword_14E64EBF0 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E64EBF0;
  }
  if ( (unsigned __int8)sub_145634460(v3, 38) )
    v2 = 10000;
  return sub_146E9FBD0(a1 + 1296, v2, 0);
}
