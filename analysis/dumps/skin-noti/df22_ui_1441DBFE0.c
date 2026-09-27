// ui_1441DBFE0

__int64 __fastcall sub_1441DBFE0(__int64 a1)
{
  __int64 v2; // rcx
  bool v3; // si
  __int64 v4; // rcx
  __int64 v5; // rax
  __int64 *v6; // rax
  __int64 v7; // rdi
  __int64 (__fastcall *v8)(__int64, __int64); // rbp
  __int64 v9; // rdx

  v3 = 1;
  if ( !(unsigned __int8)sub_146ECA4A0(*(_QWORD *)(a1 + 288))
    || !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 48))
    && !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 192))
    && !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 208))
    && !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 272))
    && !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 224))
    && !(unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 240)) )
  {
    v2 = *(_QWORD *)(a1 + 8);
    if ( !v2 || !(unsigned __int8)sub_146ED0010(v2) )
      v3 = 0;
  }
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 288))
    && !v3
    && *(int *)(a1 + 40) > 0
    && (v5 = sub_140764510(v4), (v6 = sub_1444EBD90(v5, 3, *(_DWORD *)(a1 + 40))) != 0)
    && sub_1444EC2C0((__int64)v6) )
  {
    v7 = *(_QWORD *)(a1 + 288);
    v8 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v7 + 16LL);
  }
  else
  {
    v7 = *(_QWORD *)(a1 + 288);
    v8 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v7 + 16LL);
    if ( !v3 )
    {
LABEL_21:
      v9 = 0;
      return v8(v7, v9);
    }
  }
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 304)) )
    goto LABEL_21;
  LOBYTE(v9) = 1;
  return v8(v7, v9);
}

