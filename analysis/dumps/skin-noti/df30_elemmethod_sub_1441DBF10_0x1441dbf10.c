// elemmethod_sub_1441DBF10_0x1441dbf10

__int64 __fastcall sub_1441DBF10(__int64 a1)
{
  bool v2; // cl
  __int64 *v3; // rdi
  __int64 v4; // rax
  __int64 (__fastcall *v6)(__int64 *, __int64); // rsi
  __int64 v7; // rdx

  v2 = (unsigned __int8)sub_146ECA4A0(*(_QWORD *)(a1 + 312))
    && ((unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 48))
     || (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 96))
     || (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 296))
     || (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 248))
     || (unsigned __int8)sub_146ED0010(*(_QWORD *)(a1 + 264)));
  v3 = *(__int64 **)(a1 + 312);
  v4 = *v3;
  if ( *(_BYTE *)(a1 + 392) )
    return (*(__int64 (__fastcall **)(_QWORD, bool))(v4 + 16))(*(_QWORD *)(a1 + 312), v2);
  v6 = *(__int64 (__fastcall **)(__int64 *, __int64))(v4 + 16);
  if ( !v2 || (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 328)) )
    v7 = 0;
  else
    LOBYTE(v7) = 1;
  return v6(v3, v7);
}

