// ead50caller_sub_1441BDCD0_0x1441bdcd0

_UNKNOWN **__fastcall sub_1441BDCD0(__int64 a1, _QWORD *a2)
{
  _UNKNOWN **result; // rax
  int v4; // esi
  _QWORD *v5; // rdi
  __int64 v6; // rdx
  __int64 v7; // rcx
  __int64 v8; // rax
  void (__fastcall ***v9)(_QWORD); // rcx
  __int64 v10; // rdx
  _UNKNOWN **v11; // rbx
  __int64 v12; // rax
  void (__fastcall ***v13)(_QWORD); // rcx
  __int64 v14; // rdx
  _UNKNOWN **v15; // rbp
  double v16; // xmm0_8
  unsigned int v17; // ebx
  double v18; // xmm0_8
  int v19; // eax
  _UNKNOWN *retaddr; // [rsp+78h] [rbp+0h] BYREF

  result = &retaddr;
  if ( *a2 )
  {
    v4 = 0;
    v5 = (_QWORD *)(a1 + 232);
    do
    {
      result = (_UNKNOWN **)v5[4];
      if ( (_UNKNOWN **)*a2 == result )
      {
        v6 = *((unsigned int *)v5 - 10);
        v7 = qword_14E638F28;
        if ( (_DWORD)v6 == 99999999 )
        {
          if ( !qword_14E638F28 )
          {
            v8 = sub_146E8BA20(1472);
            if ( v8 )
              v9 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v8);
            else
              v9 = 0;
            qword_14E638F28 = (__int64)v9;
            (**v9)(v9);
            v6 = *((unsigned int *)v5 - 10);
            v7 = qword_14E638F28;
          }
          result = (_UNKNOWN **)sub_1444EAD50(v7, v6, (unsigned int)v4);
          v11 = result;
          if ( result )
          {
            sub_1447EF0C0(result, v10, 0);
            sub_142757420(*v5);
            sub_146ECA0C0(*v5);
            sub_1401E41D0(v11);
            sub_1447EF290(v11);
            result = (_UNKNOWN **)sub_1447E8320(v11);
          }
        }
        else
        {
          if ( !qword_14E638F28 )
          {
            v12 = sub_146E8BA20(1472);
            if ( v12 )
              v13 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v12);
            else
              v13 = 0;
            qword_14E638F28 = (__int64)v13;
            (**v13)(v13);
            v6 = *((unsigned int *)v5 - 10);
            v7 = qword_14E638F28;
          }
          result = (_UNKNOWN **)sub_1444EAF10(v7, v6, (unsigned int)v4);
          v15 = result;
          if ( result )
          {
            sub_1447EF0F0(result, v14, 0);
            sub_14067D440(v15);
            v16 = sub_142757420(*v5);
            v17 = (int)*(float *)&v16;
            v18 = sub_146ECA0C0(*v5);
            v19 = sub_14067D440(v15);
            sub_1447EF2A0(v15, (unsigned int)(int)(float)((float)(v19 / 2) + *(float *)&v18), v17);
            result = (_UNKNOWN **)sub_1447E8BF0(v15);
          }
        }
      }
      ++v4;
      v5 += 50;
    }
    while ( v4 < 5 );
  }
  return result;
}

