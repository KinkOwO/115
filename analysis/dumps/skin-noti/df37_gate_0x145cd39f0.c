// gate_0x145cd39f0

__int64 __fastcall sub_145CD39F0(__int64 a1, int a2)
{
  __int64 v2; // rbx
  __int64 v4; // rbp
  __int64 v6; // rax
  __int64 v7; // rax
  __int64 v8; // rdi
  __int64 v9; // rsi
  __int64 v10; // rsi
  __int64 v11; // rsi
  int v12; // r8d
  int v13; // eax
  int v14; // ecx
  _DWORD *v15; // rbx
  __int64 result; // rax
  int v17; // edx
  int v18; // ecx
  void *retaddr; // [rsp+48h] [rbp+0h]
  unsigned int v20; // [rsp+50h] [rbp+8h] BYREF

  v2 = a1 + 32032;
  v4 = a1 + 44320;
  if ( a1 + 32032 == a1 + 44320 )
    return 0xFFFFFFFFLL;
  while ( 1 )
  {
    v6 = *(_QWORD *)(v2 + 8);
    if ( !v6 || !*(_DWORD *)(v6 + 8) )
      goto LABEL_25;
    v7 = *(_QWORD *)(v2 + 16);
    v8 = v7 - 16;
    if ( !v7 )
      v8 = 0;
    if ( !v8 || *(_BYTE *)(sub_140E54E40(v8) + 5179) || *(_DWORD *)(sub_140E54E40(v8) + 4880) != a2 )
      goto LABEL_25;
    v9 = sub_140E54E40(v8) + 528;
    v10 = v9 + 24LL * (*(int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4848LL))(a1);
    v11 = v10 + 8LL * *(int *)((*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 7656LL))(a1) + 16);
    sub_146E920A0(v11 - 8, &v20);
    v12 = v20;
    v13 = *(_DWORD *)(v11 - 4);
    v14 = *(_DWORD *)(v11 - 8) + v20 + 196;
    if ( v13 && v14 && v13 != v14 && retaddr )
    {
      sub_146D89B40(retaddr, v11 - 8);
      v12 = v20;
    }
    if ( v12 <= 0 )
      goto LABEL_25;
    if ( (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)a1 + 4832LL))(a1) != 10 )
      break;
    if ( (unsigned __int8)sub_145EE07F0(v8) && (unsigned __int8)sub_145EDF1F0(v8) )
    {
      v15 = (_DWORD *)sub_140E54E40(v8);
      sub_146E920A0(v15, &v20);
      result = v20;
      v17 = v20 + *v15 + 196;
      goto LABEL_19;
    }
LABEL_25:
    v2 += 24;
    if ( v2 == v4 )
      return 0xFFFFFFFFLL;
  }
  if ( (unsigned __int8)sub_145EE07F0(v8) )
    goto LABEL_25;
  v15 = (_DWORD *)sub_140E54E40(v8);
  sub_146E920A0(v15, &v20);
  result = v20;
  v17 = *v15 + v20 + 196;
LABEL_19:
  v18 = v15[1];
  if ( v18 && v17 && v18 != v17 )
  {
    if ( retaddr )
    {
      sub_146D89B40(retaddr, v15);
      return v20;
    }
  }
  return result;
}

